# p2p-drop Windows Installer (PowerShell)
# Run with: irm https://raw.githubusercontent.com/Darnix-a/p2p-file-share/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "Darnix-a/p2p-file-share"
$BinName = "p2p-drop"
$InstallDir = "$env:LOCALAPPDATA\Programs\$BinName"

Write-Host "📦 Installing $BinName for Windows..." -ForegroundColor Cyan

# Fetch latest release tag
try {
    $ReleaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Tag = $ReleaseInfo.tag_name
} catch {
    $Tag = "v1.0.0"
}

$Arch = "amd64"
$ZipName = "${BinName}-windows-${Arch}.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Tag/$ZipName"

$TempDir = Join-Path $env:TEMP ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
$ZipPath = Join-Path $TempDir $ZipName

Write-Host "⬇️  Downloading $DownloadUrl..." -ForegroundColor Yellow
Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath

Write-Host "📂 Extracting files to $InstallDir..." -ForegroundColor Yellow
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

$ExtractedExe = Join-Path $TempDir "${BinName}-windows-${Arch}.exe"
if (-not (Test-Path $ExtractedExe)) {
    $ExtractedExe = (Get-ChildItem -Path $TempDir -Filter "*.exe" -Recurse | Select-Object -First 1).FullName
}

$TargetExe = Join-Path $InstallDir "$BinName.exe"
Move-Item -Path $ExtractedExe -Destination $TargetExe -Force
Remove-Item -Path $TempDir -Recurse -Force

# Add to User PATH if not present
$UserPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "🔧 Adding $InstallDir to User PATH..." -ForegroundColor Cyan
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", [EnvironmentVariableTarget]::User)
    $env:Path += ";$InstallDir"
}

Write-Host "✅ $BinName installed successfully!" -ForegroundColor Green
Write-Host "🚀 Open a new PowerShell / Command Prompt and run: p2p-drop --help" -ForegroundColor Green
