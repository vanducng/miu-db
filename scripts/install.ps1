#Requires -Version 5.1
# Install miudb on Windows. Auto-detects architecture and fetches the matching release.
# Usage: irm https://raw.githubusercontent.com/vanducng/miu-db/main/scripts/install.ps1 | iex
# Env:   $env:MIUDB_VERSION='v0.2.4'   $env:MIUDB_INSTALL_DIR='C:\tools\miudb'
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'vanducng/miu-db'
$bin = 'miudb'
$version = if ($env:MIUDB_VERSION) { $env:MIUDB_VERSION } else { 'latest' }
$dir = if ($env:MIUDB_INSTALL_DIR) { $env:MIUDB_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'miudb\bin' }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'x86_64' }
  'ARM64' { 'arm64' }
  default {
    if ($env:PROCESSOR_ARCHITEW6432 -eq 'AMD64') { 'x86_64' }
    elseif ($env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' }
    else { throw "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
  }
}

$asset = "${bin}_windows_${arch}.zip"

# Resolve "latest" to a concrete tag so the asset and checksums come from the
# same immutable release (releases/latest/download races during a release).
if ($version -eq 'latest') {
  try {
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ 'User-Agent' = 'miudb-install' } -UseBasicParsing
    if ($rel.tag_name) { $version = $rel.tag_name }
  } catch {}
}
$base = if ($version -eq 'latest') {
  "https://github.com/$repo/releases/latest/download"
} else {
  "https://github.com/$repo/releases/download/$version"
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("miudb-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
  $zip = Join-Path $tmp $asset
  Write-Host "Downloading $asset ($version)..."
  Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip -UseBasicParsing

  $sumFile = Join-Path $tmp 'checksums.txt'
  $haveSums = $true
  try { Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $sumFile -UseBasicParsing } catch { $haveSums = $false }
  if ($haveSums) {
    $line = (Select-String -Path $sumFile -Pattern ([regex]::Escape($asset)) | Select-Object -First 1).Line
    if ($line) {
      $want = ($line -split '\s+')[0].ToLower()
      $got = (Get-FileHash -Path $zip -Algorithm SHA256).Hash.ToLower()
      if ($got -ne $want) { throw "checksum mismatch for $asset" }
    }
  }

  Expand-Archive -Path $zip -DestinationPath $tmp -Force
  $exe = Join-Path $tmp "$bin.exe"
  if (-not (Test-Path $exe)) { throw "binary $bin.exe missing from archive" }

  New-Item -ItemType Directory -Path $dir -Force | Out-Null
  Copy-Item -Path $exe -Destination (Join-Path $dir "$bin.exe") -Force

  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (-not $userPath) { $userPath = '' }
  if (($userPath -split ';') -notcontains $dir) {
    $newPath = if ($userPath) { "$userPath;$dir" } else { $dir }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    $env:Path = "$env:Path;$dir"
    Write-Host "Added $dir to your user PATH (restart your terminal to pick it up)."
  }

  Write-Host "Installed $bin -> $(Join-Path $dir "$bin.exe")"
  & (Join-Path $dir "$bin.exe") version
} finally {
  Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
