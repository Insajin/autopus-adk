$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..")).Path
. (Join-Path $RepoRoot "install.ps1") -LibraryOnly
$script:Assertions = 0

function Assert-True([bool]$Value, [string]$Message) {
    if (-not $Value) { throw "assertion failed: $Message" }
    $script:Assertions++
}

function Assert-Bytes([byte[]]$Actual, [byte[]]$Expected, [string]$Message) {
    Assert-True ($Actual.Length -eq $Expected.Length) "$Message length"
    for ($i = 0; $i -lt $Actual.Length; $i++) {
        if ($Actual[$i] -ne $Expected[$i]) { throw "assertion failed: $Message at byte $i" }
    }
    $script:Assertions++
}

function Assert-Throws([scriptblock]$Action, [string]$Expected) {
    $caught = $null
    try { & $Action } catch { $caught = $_.Exception.Message }
    if ($null -eq $caught) { throw "expected failure containing: $Expected" }
    if ($caught.IndexOf($Expected, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
        throw "unexpected failure '$caught'; wanted '$Expected'"
    }
    $script:Assertions++
}

function Get-TestHex([byte[]]$Bytes) {
    return -join ($Bytes | ForEach-Object { $_.ToString("x2") })
}

function Get-ASCIIBytes([string]$Text) {
    return [Text.Encoding]::ASCII.GetBytes($Text)
}

$FixtureDir = Join-Path $PSScriptRoot "fixtures\v0.50.73"
$checksums = [IO.File]::ReadAllBytes((Join-Path $FixtureDir "checksums.txt"))
$envelope = [IO.File]::ReadAllBytes((Join-Path $FixtureDir "checksums.txt.signatures"))
$now = [DateTime]::Parse("2026-07-18T00:00:00Z").ToUniversalTime()
$text = [Text.Encoding]::ASCII.GetString($envelope)
$lines = $text.Split([char]10)
$header = $lines[0]
$record = $lines[1]
$parts = $record.Split([char]9)
$fingerprint = $parts[0]
$encodedSignature = $parts[1]

Assert-True (Test-ReleaseSignature $checksums $envelope $ReleaseSigningKeys $now) "live K1 signature"

$spki = [Convert]::FromBase64String($ReleaseSigningKeys[0].SpkiBase64)
$ecs1 = Convert-SpkiToEcs1 $spki
$expectedEcs1 = Convert-HexBytes ("4543533120000000" +
    "0c58d8f342dcd86252b1df0ceae00efefec064add9d6c3c45eb2b81db9b89b8f" +
    "b291abef7282a5a59e6c9ff4ff9762270d7ba1c64757e9a16fe7b6e272293652")
Assert-Bytes $ecs1 $expectedEcs1 "K1 ECS1 blob"

$minimalDER = [byte[]](0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02)
$minimalP1363 = Convert-DerSignatureToP1363 $minimalDER
Assert-True ($minimalP1363.Length -eq 64) "P1363 width"
Assert-True ($minimalP1363[31] -eq 1 -and $minimalP1363[63] -eq 2) "P1363 left padding"

$highR = New-Object byte[] 32
$highR[0] = 0x80
$highDER = [byte[]](@(0x30, 0x26, 0x02, 0x21, 0x00) + @($highR) + @(0x02, 0x01, 0x01))
$highP1363 = Convert-DerSignatureToP1363 $highDER
Assert-True ($highP1363[0] -eq 0x80 -and $highP1363[63] -eq 1) "required DER sign pad"

$order = Convert-HexBytes "ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551"
$orderDER = [byte[]](@(0x30, 0x26, 0x02, 0x21, 0x00) + @($order) + @(0x02, 0x01, 0x01))
$badDER = @(
    [byte[]](0x30, 0x81, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x01),
    [byte[]](0x30, 0x07, 0x02, 0x02, 0x00, 0x01, 0x02, 0x01, 0x01),
    [byte[]](0x30, 0x06, 0x02, 0x01, 0x80, 0x02, 0x01, 0x01),
    [byte[]](0x30, 0x06, 0x02, 0x01, 0x00, 0x02, 0x01, 0x01),
    [byte[]](0x30, 0x06, 0x02, 0x01, 0x01, 0x02, 0x01, 0x01, 0x00),
    $orderDER
)
foreach ($item in $badDER) {
    Assert-Throws { Convert-DerSignatureToP1363 $item } "canonical P-256 DER signature"
}

$badSPKI = [byte[]]$spki.Clone()
$badSPKI[0] = 0x31
Assert-Throws { Convert-SpkiToEcs1 $badSPKI } "P-256 SPKI"
Assert-Throws { Convert-SpkiToEcs1 $spki[0..89] } "P-256 SPKI"

$tamperedChecksums = [byte[]]$checksums.Clone()
$tamperedChecksums[0] = $tamperedChecksums[0] -bxor 1
Assert-Throws {
    Test-ReleaseSignature $tamperedChecksums $envelope $ReleaseSigningKeys $now
} "no trusted release signing key verified"

$signatureDER = [Convert]::FromBase64String($encodedSignature)
$signatureDER[$signatureDER.Length - 1] = $signatureDER[$signatureDER.Length - 1] -bxor 1
$badSignatureRecord = $fingerprint + "`t" + [Convert]::ToBase64String($signatureDER)
$badSignatureEnvelope = Get-ASCIIBytes ("$header`n$badSignatureRecord`n")
Assert-Throws {
    Test-ReleaseSignature $checksums $badSignatureEnvelope $ReleaseSigningKeys $now
} "no trusted release signing key verified"

$unknownRecord = ("0" * 64) + "`t" + $encodedSignature
$unknownEnvelope = Get-ASCIIBytes ("$header`n$unknownRecord`n")
Assert-Throws {
    Test-ReleaseSignature $checksums $unknownEnvelope $ReleaseSigningKeys $now
} "no trusted release signing key verified"

$duplicateEnvelope = Get-ASCIIBytes ("$header`n$record`n$record`n")
$oversized = New-Object byte[] 4097
$nulled = [byte[]]$envelope.Clone()
$nulled[5] = 0
$bom = [byte[]](@(0xef, 0xbb, 0xbf) + @($envelope))
$many = Get-ASCIIBytes ($header + "`n" + (($record + "`n") * 17))
$longRecord = ("0" * 64) + "`t" + ("A" * 192)
$malformed = @(
    [byte[]]@(),
    $oversized,
    (Get-ASCIIBytes "WRONG`n$record`n"),
    (Get-ASCIIBytes ($text.Replace("`n", "`r`n"))),
    (Get-ASCIIBytes $text.Substring(0, $text.Length - 1)),
    $bom,
    $nulled,
    (Get-ASCIIBytes "$header`n`n"),
    $many,
    (Get-ASCIIBytes "$header`n$longRecord`n"),
    (Get-ASCIIBytes ("$header`n" + $record.ToUpperInvariant() + "`n")),
    (Get-ASCIIBytes ("$header`n" + $record.Replace("`t", " ") + "`n")),
    (Get-ASCIIBytes ("$header`n" + ("0" * 64) + "`t!`n$record`n")),
    $duplicateEnvelope
)
foreach ($item in $malformed) {
    Assert-Throws {
        Test-ReleaseSignature $checksums $item $ReleaseSigningKeys $now
    } "malformed release signature envelope"
}

$expiredKeys = @([pscustomobject]@{
    Fingerprint = $fingerprint; ExpiresAt = "2020-01-01"; SpkiBase64 = $ReleaseSigningKeys[0].SpkiBase64
})
Assert-Throws {
    Test-ReleaseSignature $checksums $envelope $expiredKeys $now
} "all embedded release signing keys expired"
$badKeys = @([pscustomobject]@{
    Fingerprint = ("0" * 64); ExpiresAt = "2099-12-31"; SpkiBase64 = $ReleaseSigningKeys[0].SpkiBase64
})
Assert-Throws {
    Test-ReleaseSignature $checksums $envelope $badKeys $now
} "malformed embedded release signing key"
$badPoint = [byte[]]$spki.Clone()
$badPoint[$badPoint.Length - 1] = $badPoint[$badPoint.Length - 1] -bxor 1
$malformedKeys = @(
    @([pscustomobject]@{ Fingerprint = $fingerprint; ExpiresAt = "2026-99-99"; SpkiBase64 = $ReleaseSigningKeys[0].SpkiBase64 }),
    @([pscustomobject]@{ Fingerprint = (Get-Sha256Hex $badPoint); ExpiresAt = "2099-12-31"; SpkiBase64 = [Convert]::ToBase64String($badPoint) }),
    @([pscustomobject]@{ Fingerprint = $fingerprint; ExpiresAt = "2099-12-31"; SpkiBase64 = $ReleaseSigningKeys[0].SpkiBase64 + " " })
)
foreach ($keys in $malformedKeys) {
    Assert-Throws { Test-ReleaseSignature $checksums $envelope $keys $now } "malformed embedded release signing key"
}
Assert-Throws {
    Test-ReleaseSignature -Checksums $checksums -Envelope $envelope -Keys @() -Now $now
} "malformed embedded release signing key"
$duplicateKeys = @($ReleaseSigningKeys[0], $ReleaseSigningKeys[0])
Assert-Throws {
    Test-ReleaseSignature $checksums $envelope $duplicateKeys $now
} "malformed embedded release signing key"

# Exercise Main's no-install boundary without network or filesystem mutation outside a temp root.
$sandbox = Join-Path ([IO.Path]::GetTempPath()) "autopus-ps-signing-$([Guid]::NewGuid())"
[void](New-Item -ItemType Directory -Path $sandbox)
$savedTemp = $env:TEMP
$savedVersion = $env:VERSION
$savedArch = $env:PROCESSOR_ARCHITECTURE
$env:TEMP = $sandbox
$env:VERSION = "0.50.73"
$env:PROCESSOR_ARCHITECTURE = "AMD64"
$InstallDir = Join-Path $sandbox "install"
$script:ArchiveDownloads = 0
$script:InstallActions = 0
$script:MockChecksums = $checksums
$script:MockEnvelope = $unknownEnvelope
function Err($msg) { throw [System.InvalidOperationException]::new([string]$msg) }
function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
    if ($Uri.EndsWith("checksums.txt.signatures")) {
        [IO.File]::WriteAllBytes($OutFile, $script:MockEnvelope)
    } elseif ($Uri.EndsWith("checksums.txt")) {
        [IO.File]::WriteAllBytes($OutFile, $script:MockChecksums)
    } else {
        $script:ArchiveDownloads++
        [IO.File]::WriteAllBytes($OutFile, [byte[]](1, 2, 3))
    }
}
function Expand-Archive { param($Path, $DestinationPath, [switch]$Force); $script:InstallActions++ }
function Copy-Item { param($Path, $Destination, [switch]$Force); $script:InstallActions++ }
try {
    $env:VERSION = "0.50.72"
    Assert-Throws { Main } "unsigned_release_not_supported"
    Assert-True ($script:ArchiveDownloads -eq 0) "unsigned floor blocks all downloads"
    $env:VERSION = "0.50.73"
    Assert-Throws { Main } "no trusted release signing key verified"
    Assert-True ($script:ArchiveDownloads -eq 0) "archive download blocked before signature verification"
    Assert-True ($script:InstallActions -eq 0) "extract/copy blocked on signature failure"
    Assert-True (-not (Test-Path (Join-Path $InstallDir "auto.exe"))) "no installed binary on failure"

    $script:SignatureChecks = 0
    $script:ChecksumChecks = 0
    $script:MockChecksums = Get-ASCIIBytes (("0" * 64) + "  autopus-adk_0.50.73_windows_amd64.zip`n")
    function Test-ReleaseSignature { param($Checksums, $Envelope); $script:SignatureChecks++; return $true }
    function Verify-Checksum { param($file, $expected); $script:ChecksumChecks++ }
    function Add-InstallerPath { param($Dir); return $false }
    function Expand-Archive {
        param($Path, $DestinationPath, [switch]$Force)
        $script:InstallActions++; [IO.File]::WriteAllBytes((Join-Path $DestinationPath "auto.exe"), [byte[]](1, 2, 3))
    }
    function Copy-Item {
        param($Path, $Destination, [switch]$Force)
        $script:InstallActions++; [IO.File]::WriteAllBytes($Destination, [byte[]](1, 2, 3))
    }
    Main
    Assert-True ($script:SignatureChecks -eq 1) "normal path verifies signature once"
    Assert-True ($script:ArchiveDownloads -eq 1) "normal path downloads one archive"
    Assert-True ($script:ChecksumChecks -eq 1) "normal path verifies checksum once"
    Assert-True ($script:InstallActions -eq 3) "normal path extracts and copies both commands"
    Assert-True (Test-Path (Join-Path $InstallDir "auto.exe")) "normal path installs auto.exe"
    Assert-True (Test-Path (Join-Path $InstallDir "autopus.exe")) "normal path installs alias"
} finally {
    $env:TEMP = $savedTemp
    $env:VERSION = $savedVersion
    $env:PROCESSOR_ARCHITECTURE = $savedArch
    Remove-Item $sandbox -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "PASS windows installer signing ($($PSVersionTable.PSVersion); $script:Assertions assertions)"
