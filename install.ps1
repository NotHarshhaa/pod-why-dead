# PowerShell install script for pod-why-dead

$ErrorActionPreference = "Stop"

# Detect architecture
if ([Environment]::Is64BitOperatingSystem) {
    $ARCH = "amd64"
} else {
    Write-Error "32-bit Windows is not supported"
    exit 1
}

# Get latest version
$LatestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/NotHarshhaa/pod-why-dead/releases/latest"
$Version = $LatestRelease.tag_name

if (-not $Version) {
    Write-Error "Failed to fetch latest version"
    exit 1
}

Write-Host "Installing pod-why-dead $Version for windows-$ARCH..."

# Download
$DownloadUrl = "https://github.com/NotHarshhaa/pod-why-dead/releases/download/${Version}/pod-why-dead_${Version}_windows_${ARCH}.zip"
$TmpDir = Join-Path $env:TEMP "pod-why-dead-install"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null
$ZipPath = Join-Path $TmpDir "pod-why-dead.zip"

Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing

# Extract
Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force

# Install
$InstallDir = "$env:USERPROFILE\AppData\Local\Programs\pod-why-dead"
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$ExePath = Join-Path $TmpDir "pod-why-dead.exe"
Copy-Item -Path $ExePath -Destination $InstallDir -Force

# Add to PATH if not already there
$PathEnv = [Environment]::GetEnvironmentVariable("Path", "User")
if ($PathEnv -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$PathEnv;$InstallDir", "User")
    Write-Host "Added $InstallDir to user PATH. Please restart your terminal."
}

# Cleanup
Remove-Item -Path $TmpDir -Recurse -Force

Write-Host "Successfully installed pod-why-dead to $InstallDir\pod-why-dead.exe"
