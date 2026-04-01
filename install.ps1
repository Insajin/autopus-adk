# autopus-adk Windows install script
# Usage: irm https://raw.githubusercontent.com/Insajin/autopus-adk/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "Insajin/autopus-adk"
$Binary = "auto.exe"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\autopus-adk\bin" }

function Info($msg)  { Write-Host $msg -ForegroundColor Cyan }
function Ok($msg)    { Write-Host $msg -ForegroundColor Green }
function Err($msg)   { Write-Host $msg -ForegroundColor Red; exit 1 }

# Detect architecture
function Get-Arch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64"   { return "amd64" }
        "Arm64" { return "arm64" }
        default { Err "Unsupported architecture: $arch" }
    }
}

# Get latest version from GitHub API
function Get-LatestVersion {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "autopus-installer" }
    return $release.tag_name -replace '^v', ''
}

# Verify SHA256 checksum
function Verify-Checksum($file, $expected) {
    $actual = (Get-FileHash -Path $file -Algorithm SHA256).Hash.ToLower()
    if ($actual -ne $expected) {
        Err "Checksum mismatch!`n  expected: $expected`n  actual:   $actual"
    }
}

function Main {
    $Arch = Get-Arch
    $Version = if ($env:VERSION) { $env:VERSION } else { Get-LatestVersion }

    if (-not $Version) {
        Err "Failed to get latest version. Check GitHub API limits."
    }

    Info "autopus-adk v$Version installing... (windows/$Arch)"

    $Archive = "autopus-adk_${Version}_windows_${Arch}.zip"
    $BaseUrl = "https://github.com/$Repo/releases/download/v$Version"
    $Url = "$BaseUrl/$Archive"
    $ChecksumsUrl = "$BaseUrl/checksums.txt"

    $TmpDir = Join-Path $env:TEMP "autopus-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

    try {
        Info "Downloading: $Url"
        Invoke-WebRequest -Uri $Url -OutFile "$TmpDir\$Archive" -UseBasicParsing

        # SHA256 checksum verification
        Info "Verifying checksum..."
        Invoke-WebRequest -Uri $ChecksumsUrl -OutFile "$TmpDir\checksums.txt" -UseBasicParsing
        $checksumLine = Get-Content "$TmpDir\checksums.txt" | Where-Object { $_ -match $Archive }
        if ($checksumLine) {
            $expected = ($checksumLine -split '\s+')[0].ToLower()
            Verify-Checksum "$TmpDir\$Archive" $expected
            Ok "Checksum verified"
        } else {
            Err "Checksum not found for $Archive in checksums.txt"
        }

        Info "Extracting..."
        Expand-Archive -Path "$TmpDir\$Archive" -DestinationPath $TmpDir -Force

        # Create install directory if it doesn't exist
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }

        Info "Installing to $InstallDir\$Binary..."
        Copy-Item "$TmpDir\auto.exe" "$InstallDir\$Binary" -Force

        # Add to PATH if not already present
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
            $env:Path = "$env:Path;$InstallDir"
            Info "Added $InstallDir to user PATH"
        }

        Ok "autopus-adk v$Version installed!"
        Ok ""

        # Post-install: check and install dependencies (skip already installed)
        Info "Checking dependencies..."
        try {
            & "$InstallDir\$Binary" doctor --fix --yes 2>$null
            Ok "Dependencies installed!"
        } catch {
            Write-Host "  Some dependencies could not be auto-installed." -ForegroundColor Yellow
            Write-Host "  Run manually: auto doctor" -ForegroundColor Yellow
        }

        Ok ""
        Ok "  Restart your terminal, then run:"
        Ok "    auto version     # verify install"
        Ok "    auto init        # initialize project"
        Ok ""
    }
    finally {
        Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Main
