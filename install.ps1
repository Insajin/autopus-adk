# autopus-adk Windows install script
# Usage: irm https://raw.githubusercontent.com/Insajin/autopus-adk/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "Insajin/autopus-adk"
$Binary = "auto.exe"
$AliasBinary = "autopus.exe"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\autopus-adk\bin" }

function Info($msg)  { Write-Host $msg -ForegroundColor Cyan }
function Ok($msg)    { Write-Host $msg -ForegroundColor Green }
function Err($msg)   { Write-Host $msg -ForegroundColor Red; exit 1 }

function Test-PathContainsDir([string]$PathValue, [string]$Dir) {
    if (-not $PathValue) { return $false }
    $needle = $Dir.TrimEnd('\').ToLowerInvariant()
    foreach ($entry in ($PathValue -split ';')) {
        if ($entry.TrimEnd('\').ToLowerInvariant() -eq $needle) {
            return $true
        }
    }
    return $false
}

function Get-GitBashPath([string]$Dir) {
    if ($Dir -match '^([A-Za-z]):\\(.*)$') {
        $drive = $matches[1].ToLowerInvariant()
        $rest = ($matches[2] -replace '\\', '/')
        return "/$drive/$rest"
    }
    return $Dir -replace '\\', '/'
}

function Show-PathHint([string]$InstallDir, [bool]$PathAdded) {
    Ok "  Installed commands:"
    Ok "    auto"
    Ok "    autopus    # auto alias"

    if (Test-PathContainsDir $env:Path $InstallDir) {
        Ok "  PATH ready in this PowerShell session: $InstallDir"
    } else {
        Write-Host "  PATH was updated for future shells, but this parent shell may need a restart." -ForegroundColor Yellow
    }

    if ($PathAdded) {
        Write-Host "  New terminals will pick up: $InstallDir" -ForegroundColor Yellow
    }

    if ($env:MSYSTEM) {
        $bashPath = Get-GitBashPath $InstallDir
        Write-Host "  Git Bash에서 설치했다면 현재 창을 다시 열거나 아래를 실행하세요:" -ForegroundColor Yellow
        Write-Host "    export PATH=""$bashPath:`$PATH""" -ForegroundColor Yellow
    }
}

# Detect architecture
function Get-Arch {
    $envArch = $env:PROCESSOR_ARCHITECTURE
    switch ($envArch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { Err "Unsupported architecture: $envArch" }
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
        $TargetPath = "$InstallDir\$Binary"
        $OldPath = "$TargetPath.old"
        if (Test-Path $TargetPath) {
            # Running exe cannot be overwritten but CAN be renamed.
            Remove-Item $OldPath -Force -ErrorAction SilentlyContinue
            try {
                Rename-Item $TargetPath $OldPath -Force
            } catch {
                # Rename failed — try direct copy as last resort.
            }
        }
        Copy-Item "$TmpDir\auto.exe" $TargetPath -Force
        Copy-Item "$TmpDir\auto.exe" "$InstallDir\$AliasBinary" -Force
        Remove-Item $OldPath -Force -ErrorAction SilentlyContinue

        # Add to PATH if not already present
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        $pathAdded = $false
        if (-not (Test-PathContainsDir $userPath $InstallDir)) {
            $newUserPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
            [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
            Info "Added $InstallDir to user PATH"
            $pathAdded = $true
        }
        if (-not (Test-PathContainsDir $env:Path $InstallDir)) {
            $env:Path = if ($env:Path) { "$env:Path;$InstallDir" } else { $InstallDir }
        }

        Ok "autopus-adk v$Version installed!"
        Ok ""
        Show-PathHint $InstallDir $pathAdded
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

        # Auto-init: initialize harness
        if ($env:SKIP_INIT -eq "1") {
            Ok ""
            Ok "  SKIP_INIT=1 — skipping initialization."
            Ok "  Next: auto init"
            Ok ""
        }
        elseif ((Test-Path "CLAUDE.md") -or (Test-Path "autopus.yaml")) {
            Ok "Already initialized. Running update..."
            try { & "$InstallDir\$Binary" update --yes 2>$null } catch {}
            Ok ""
            Ok "  Ready to use:"
            Ok "    /auto setup    # generate project context"
            Ok "    /auto status   # SPEC dashboard"
            Ok ""
        }
        else {
            Info "Initializing project..."
            $Proj = if ($env:PROJECT_NAME) { $env:PROJECT_NAME } else { Split-Path -Leaf (Get-Location) }
            Info "  Project: $Proj"
            try {
                if ($env:PLATFORMS) {
                    Info "  Platforms: $env:PLATFORMS"
                    & "$InstallDir\$Binary" init --project $Proj --platforms $env:PLATFORMS --yes 2>&1
                } else {
                    Info "  Platforms: auto-detect"
                    & "$InstallDir\$Binary" init --project $Proj --yes 2>&1
                }
                Ok "Project initialized!"
            } catch {
                Write-Host "  Init failed. Run manually: auto init" -ForegroundColor Yellow
            }
            Ok ""
            Ok "  Ready to use in Claude Code:"
            Ok "    /auto setup    # generate project context"
            Ok "    /auto plan     # write a SPEC"
            Ok "    /auto fix      # fix a bug"
            Ok "    /auto review   # code review"
            Ok ""
        }
    }
    finally {
        Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Main
