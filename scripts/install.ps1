# Installs the latest (or a pinned) cloudtui release binary for Windows.
# No admin rights required, no PATH/registry edits — extracts into a
# per-user directory and prints a hint if that directory isn't already on
# PATH.
#
# Usage:
#   irm https://raw.githubusercontent.com/ePex/cloudtui/main/scripts/install.ps1 | iex
#
# Env vars:
#   CLOUDTUI_VERSION     release tag to install, e.g. v0.3.0 (default: latest)
#   CLOUDTUI_INSTALL_DIR directory to install the binary into
#                        (default: $env:LOCALAPPDATA\cloudtui\bin)
#
# PowerShell 5.1+ compatible (works on a stock Windows install, no
# PowerShell 7 requirement). See scripts/install.sh for macOS/Linux.

$ErrorActionPreference = 'Stop'

$Repo = 'ePex/cloudtui'
$InstallDir = $env:CLOUDTUI_INSTALL_DIR
if ([string]::IsNullOrEmpty($InstallDir)) {
	$localAppData = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
	$InstallDir = Join-Path $localAppData 'cloudtui\bin'
}

switch ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture) {
	'X64' { $arch = 'amd64' }
	'Arm64' { $arch = 'arm64' }
	default {
		Write-Error "install.ps1: unsupported architecture: $_"
		exit 1
	}
}

$tag = $env:CLOUDTUI_VERSION
if ([string]::IsNullOrEmpty($tag)) {
	$req = [System.Net.HttpWebRequest]::Create("https://github.com/$Repo/releases/latest")
	$req.AllowAutoRedirect = $false
	$req.Method = 'HEAD'
	$resp = $req.GetResponse()
	$location = $resp.Headers['Location']
	$resp.Close()
	if ([string]::IsNullOrEmpty($location)) {
		Write-Error 'install.ps1: could not resolve the latest release tag'
		exit 1
	}
	$tag = $location.Split('/')[-1]
}
$version = $tag -replace '^v', ''

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmpDir | Out-Null
try {
	$archive = "cloudtui_${version}_windows_${arch}.zip"
	$baseUrl = "https://github.com/$Repo/releases/download/$tag"
	$archivePath = Join-Path $tmpDir $archive
	$checksumsPath = Join-Path $tmpDir 'checksums.txt'

	Write-Host "install.ps1: downloading $archive ($tag)..."
	Invoke-WebRequest -Uri "$baseUrl/$archive" -OutFile $archivePath -UseBasicParsing
	Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksumsPath -UseBasicParsing

	$checksumLine = Select-String -Path $checksumsPath -Pattern ([regex]::Escape($archive)) | Select-Object -First 1
	if (-not $checksumLine) {
		Write-Error "install.ps1: no checksum entry for $archive in checksums.txt"
		exit 1
	}
	$expectedHash = ($checksumLine.Line -split '\s+')[0]
	$actualHash = (Get-FileHash -Algorithm SHA256 -Path $archivePath).Hash
	if ($actualHash.ToLower() -ne $expectedHash.ToLower()) {
		Write-Error "install.ps1: checksum verification failed for $archive"
		exit 1
	}

	$extractDir = Join-Path $tmpDir 'extracted'
	Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

	New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
	Copy-Item -Path (Join-Path $extractDir 'cloudtui.exe') -Destination (Join-Path $InstallDir 'cloudtui.exe') -Force

	Write-Host "install.ps1: installed cloudtui $tag to $InstallDir\cloudtui.exe"

	$pathEntries = $env:Path -split ';'
	if ($pathEntries -notcontains $InstallDir) {
		Write-Host "install.ps1: $InstallDir is not on your PATH."
		Write-Host "  Add it, e.g.: `$env:Path = `"$InstallDir;`$env:Path`""
	}
}
finally {
	Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
