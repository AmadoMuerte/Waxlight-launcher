param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$OutputDirectory = "release"
)

$ErrorActionPreference = "Stop"
foreach ($RequiredCommand in @("node", "npm", "wails")) {
    if (-not (Get-Command $RequiredCommand -ErrorAction SilentlyContinue)) {
        throw "Required build command is unavailable: $RequiredCommand"
    }
}

$ReleaseVersion = $Version.TrimStart("v")
if ($ReleaseVersion -notmatch '^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
    throw "Invalid release version: $Version"
}

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$RootConfig = Get-Content (Join-Path $ProjectRoot "wails.json") -Raw | ConvertFrom-Json
$CommandConfig = Get-Content (Join-Path $ProjectRoot "cmd/waxlight/wails.json") -Raw | ConvertFrom-Json
if ($RootConfig.info.productVersion -ne $ReleaseVersion -or $CommandConfig.info.productVersion -ne $ReleaseVersion) {
    throw "Release version does not match both wails.json files"
}

$ResolvedOutput = [System.IO.Path]::GetFullPath((Join-Path $ProjectRoot $OutputDirectory))
New-Item -ItemType Directory -Force -Path $ResolvedOutput | Out-Null

npm ci --include=dev --prefix (Join-Path $ProjectRoot "frontend")
npm --prefix (Join-Path $ProjectRoot "frontend") run build

Push-Location (Join-Path $ProjectRoot "cmd/waxlight")
$OriginalAppData = $env:APPDATA
$BuildAppData = Join-Path ([System.IO.Path]::GetTempPath()) ("waxlight-build-config-" + [guid]::NewGuid())
New-Item -ItemType Directory -Force -Path $BuildAppData | Out-Null
try {
    $env:APPDATA = $BuildAppData
    wails build -clean -platform windows/amd64 -nsis
} finally {
    $env:APPDATA = $OriginalAppData
    Remove-Item $BuildAppData -Recurse -Force -ErrorAction SilentlyContinue
    Pop-Location
}

$BuildDirectory = Join-Path $ProjectRoot "build/bin"
$PortableBinary = Join-Path $BuildDirectory "waxlight.exe"
if (-not (Test-Path $PortableBinary)) {
    throw "Wails did not produce $PortableBinary"
}

$AssetPrefix = "Waxlight-Launcher-v$ReleaseVersion-windows-amd64"
$PortableAsset = Join-Path $ResolvedOutput "$AssetPrefix.exe"
Copy-Item $PortableBinary $PortableAsset -Force

$Staging = Join-Path ([System.IO.Path]::GetTempPath()) ("waxlight-portable-" + [guid]::NewGuid())
New-Item -ItemType Directory -Force -Path $Staging | Out-Null
try {
    Copy-Item $PortableBinary (Join-Path $Staging "waxlight.exe")
    Copy-Item (Join-Path $ProjectRoot "README.md") $Staging
    Copy-Item (Join-Path $ProjectRoot "LICENSE") $Staging
    Copy-Item (Join-Path $ProjectRoot "NOTICE") $Staging
    Compress-Archive -Path (Join-Path $Staging "*") -DestinationPath (Join-Path $ResolvedOutput "$AssetPrefix-portable.zip") -Force
} finally {
    Remove-Item $Staging -Recurse -Force -ErrorAction SilentlyContinue
}

$Installer = Get-ChildItem -Path $BuildDirectory -Filter "*installer*.exe" | Select-Object -First 1
if (-not $Installer) {
    throw "Wails did not produce an NSIS installer in $BuildDirectory"
}
Copy-Item $Installer.FullName (Join-Path $ResolvedOutput "$AssetPrefix-installer.exe") -Force

Write-Host "Windows packages written to $ResolvedOutput"
