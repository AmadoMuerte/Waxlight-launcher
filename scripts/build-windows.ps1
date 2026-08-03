param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [string]$OutputDirectory = "release"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Assert-Command {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $Command = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $Command) {
        throw "Required build command is unavailable: $Name"
    }

    return $Command
}

function Assert-FileExists {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        [string]$Description
    )

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Description was not found: $Path"
    }

    $Item = Get-Item -LiteralPath $Path
    if ($Item.Length -le 0) {
        throw "$Description is empty: $Path"
    }
}

function Invoke-CheckedCommand {
    param(
        [Parameter(Mandatory = $true)]
        [scriptblock]$Command,

        [Parameter(Mandatory = $true)]
        [string]$Description
    )

    & $Command

    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE"
    }
}

$RequiredCommands = @(
    "node",
    "npm",
    "go",
    "wails",
    "makensis",
    "gcc",
    "g++"
)

foreach ($RequiredCommand in $RequiredCommands) {
    Assert-Command -Name $RequiredCommand | Out-Null
}

$ReleaseVersion = $Version.TrimStart("v")

if ($ReleaseVersion -notmatch '^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
    throw "Invalid release version: $Version"
}

if ($env:CGO_ENABLED -ne "1") {
    throw "Windows release build requires CGO_ENABLED=1 because go-sqlite3 uses CGO"
}

if ([string]::IsNullOrWhiteSpace($env:CC)) {
    $env:CC = "gcc"
}

if ([string]::IsNullOrWhiteSpace($env:CXX)) {
    $env:CXX = "g++"
}

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$FrontendDirectory = Join-Path $ProjectRoot "frontend"
$FrontendIndex = Join-Path $FrontendDirectory "dist/index.html"
$CommandDirectory = Join-Path $ProjectRoot "cmd/waxlight"
$BuildDirectory = Join-Path $ProjectRoot "build/bin"

$RootConfigPath = Join-Path $ProjectRoot "wails.json"
$CommandConfigPath = Join-Path $CommandDirectory "wails.json"

Assert-FileExists -Path $RootConfigPath -Description "Root Wails configuration"
Assert-FileExists -Path $CommandConfigPath -Description "Command Wails configuration"

$RootConfig = Get-Content -LiteralPath $RootConfigPath -Raw | ConvertFrom-Json
$CommandConfig = Get-Content -LiteralPath $CommandConfigPath -Raw | ConvertFrom-Json

if ($RootConfig.info.productVersion -ne $ReleaseVersion) {
    throw "Version mismatch in $RootConfigPath. Expected $ReleaseVersion, got $($RootConfig.info.productVersion)"
}

if ($CommandConfig.info.productVersion -ne $ReleaseVersion) {
    throw "Version mismatch in $CommandConfigPath. Expected $ReleaseVersion, got $($CommandConfig.info.productVersion)"
}

$ResolvedOutput = [System.IO.Path]::GetFullPath(
    (Join-Path $ProjectRoot $OutputDirectory)
)

New-Item `
    -ItemType Directory `
    -Force `
    -Path $ResolvedOutput |
    Out-Null

Write-Host ""
Write-Host "Waxlight Launcher Windows release build"
Write-Host "Version:        $ReleaseVersion"
Write-Host "Project root:   $ProjectRoot"
Write-Host "Output:         $ResolvedOutput"
Write-Host "CGO_ENABLED:    $env:CGO_ENABLED"
Write-Host "CC:             $env:CC"
Write-Host "CXX:            $env:CXX"
Write-Host ""

Invoke-CheckedCommand `
    -Description "Go environment check" `
    -Command {
        go env GOOS GOARCH CGO_ENABLED CC CXX
    }

Invoke-CheckedCommand `
    -Description "GCC version check" `
    -Command {
        gcc --version
    }

Invoke-CheckedCommand `
    -Description "G++ version check" `
    -Command {
        g++ --version
    }

Invoke-CheckedCommand `
    -Description "Wails version check" `
    -Command {
        wails version
    }

Invoke-CheckedCommand `
    -Description "NSIS version check" `
    -Command {
        makensis /VERSION
    }

Write-Host ""
Write-Host "Installing frontend dependencies..."

Invoke-CheckedCommand `
    -Description "Frontend dependency installation" `
    -Command {
        npm ci `
            --include=dev `
            --prefix $FrontendDirectory
    }

Write-Host ""
Write-Host "Building frontend..."

Invoke-CheckedCommand `
    -Description "Frontend build" `
    -Command {
        npm `
            --prefix $FrontendDirectory `
            run build
    }

Assert-FileExists `
    -Path $FrontendIndex `
    -Description "Frontend production entrypoint"

Write-Host ""
Write-Host "Building Windows application..."

$OriginalAppData = $env:APPDATA
$BuildAppData = Join-Path `
    ([System.IO.Path]::GetTempPath()) `
    ("waxlight-build-config-" + [guid]::NewGuid())

New-Item `
    -ItemType Directory `
    -Force `
    -Path $BuildAppData |
    Out-Null

try {
    $env:APPDATA = $BuildAppData

    Push-Location $CommandDirectory

    try {
        Invoke-CheckedCommand `
            -Description "Wails Windows release build" `
            -Command {
                wails build `
                    -clean `
                    -platform windows/amd64 `
                    -nsis `
                    -trimpath `
                    -ldflags="-s -w"
            }
    }
    finally {
        Pop-Location
    }
}
finally {
    $env:APPDATA = $OriginalAppData

    Remove-Item `
        -LiteralPath $BuildAppData `
        -Recurse `
        -Force `
        -ErrorAction SilentlyContinue
}

$PortableBinary = Join-Path $BuildDirectory "waxlight.exe"

Assert-FileExists `
    -Path $PortableBinary `
    -Description "Wails Windows executable"

$AssetPrefix = "Waxlight-Launcher-v$ReleaseVersion-windows-amd64"

$PortableAsset = Join-Path `
    $ResolvedOutput `
    "$AssetPrefix.exe"

Copy-Item `
    -LiteralPath $PortableBinary `
    -Destination $PortableAsset `
    -Force

Assert-FileExists `
    -Path $PortableAsset `
    -Description "Portable Windows executable"

Write-Host ""
Write-Host "Creating portable archive..."

$Staging = Join-Path `
    ([System.IO.Path]::GetTempPath()) `
    ("waxlight-portable-" + [guid]::NewGuid())

New-Item `
    -ItemType Directory `
    -Force `
    -Path $Staging |
    Out-Null

try {
    Copy-Item `
        -LiteralPath $PortableBinary `
        -Destination (Join-Path $Staging "waxlight.exe") `
        -Force

    $DocumentationFiles = @(
        "README.md",
        "LICENSE",
        "NOTICE"
    )

    foreach ($DocumentationFile in $DocumentationFiles) {
        $SourcePath = Join-Path $ProjectRoot $DocumentationFile

        Assert-FileExists `
            -Path $SourcePath `
            -Description $DocumentationFile

        Copy-Item `
            -LiteralPath $SourcePath `
            -Destination $Staging `
            -Force
    }

    $PortableArchive = Join-Path `
        $ResolvedOutput `
        "$AssetPrefix-portable.zip"

    Compress-Archive `
        -Path (Join-Path $Staging "*") `
        -DestinationPath $PortableArchive `
        -Force

    Assert-FileExists `
        -Path $PortableArchive `
        -Description "Portable Windows archive"
}
finally {
    Remove-Item `
        -LiteralPath $Staging `
        -Recurse `
        -Force `
        -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "Locating NSIS installer..."

$Installer = Get-ChildItem `
    -LiteralPath $BuildDirectory `
    -Filter "*.exe" `
    -File |
    Where-Object {
        $_.Name -match "installer|setup"
    } |
    Sort-Object LastWriteTimeUtc -Descending |
    Select-Object -First 1

if (-not $Installer) {
    Write-Host "Contents of build directory:"
    Get-ChildItem -LiteralPath $BuildDirectory |
        Format-Table Name, Length, LastWriteTime

    throw "Wails did not produce an NSIS installer in $BuildDirectory"
}

$InstallerAsset = Join-Path `
    $ResolvedOutput `
    "$AssetPrefix-installer.exe"

Copy-Item `
    -LiteralPath $Installer.FullName `
    -Destination $InstallerAsset `
    -Force

Assert-FileExists `
    -Path $InstallerAsset `
    -Description "Windows NSIS installer"

Write-Host ""
Write-Host "Windows packages written to $ResolvedOutput"
Write-Host ""

Get-ChildItem `
    -LiteralPath $ResolvedOutput `
    -File |
    Where-Object {
        $_.Name -like "$AssetPrefix*"
    } |
    Sort-Object Name |
    Format-Table Name, Length, LastWriteTimeUtc
