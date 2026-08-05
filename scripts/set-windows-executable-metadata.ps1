#Requires -Version 5.1

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Test-MetadataValue {
    param(
        [Parameter(Mandatory = $true)]
        [System.Collections.Generic.List[string]]$Errors,

        [Parameter(Mandatory = $true)]
        [string]$Name,

        [AllowNull()]
        [string]$Actual,

        [Parameter(Mandatory = $true)]
        [string]$Expected
    )

    if ($Actual -ne $Expected) {
        $Errors.Add("$Name must be '$Expected', got '$Actual'")
    }
}

$ResolvedExecutable = (Resolve-Path -LiteralPath $ExecutablePath).Path
if (-not (Test-Path -LiteralPath $ResolvedExecutable -PathType Leaf)) {
    throw "Windows executable not found: $ExecutablePath"
}

$RepositoryRoot = Split-Path -Parent $PSScriptRoot
$WailsConfigPath = Join-Path $RepositoryRoot "cmd\waxlight\wails.json"
if (-not (Test-Path -LiteralPath $WailsConfigPath -PathType Leaf)) {
    throw "Wails configuration not found: $WailsConfigPath"
}

$Config = Get-Content -LiteralPath $WailsConfigPath -Raw -Encoding UTF8 |
    ConvertFrom-Json
$WindowsVersion = [string]$Config.info.productVersion

if ($WindowsVersion -notmatch '^\d+\.\d+\.\d+\.\d+$') {
    throw "Windows product version must be numeric x.x.x.x, got '$WindowsVersion'"
}

$CopyrightSymbol = [char]0x00A9
$ExpectedCopyright = "Copyright $CopyrightSymbol 2026 AmadoMuerte"
$ExpectedComments =
    "Free software licensed under the GNU General Public License v3.0."

$ResEdit = Get-Command "resedit" -ErrorAction SilentlyContinue
if ($null -eq $ResEdit) {
    $ResEdit = Get-Command "resedit.cmd" -ErrorAction SilentlyContinue
}
if ($null -eq $ResEdit) {
    throw "resedit-cli is not installed or is not available in PATH"
}

$ExecutableDirectory = Split-Path -Parent $ResolvedExecutable
$ExecutableName = Split-Path -Leaf $ResolvedExecutable
$TemporaryExecutable = Join-Path `
    $ExecutableDirectory `
    ("." + $ExecutableName + ".metadata-" + [Guid]::NewGuid().ToString("N") + ".tmp.exe")
$TemporaryDirectory = Join-Path `
    ([System.IO.Path]::GetTempPath()) `
    ("waxlight-metadata-" + [Guid]::NewGuid().ToString("N"))
$DefinitionPath = Join-Path $TemporaryDirectory "version-info.json"

try {
    New-Item -ItemType Directory -Path $TemporaryDirectory -Force | Out-Null

    $Definition = [ordered]@{
        lang = 1033
        version = [ordered]@{
            comments = $ExpectedComments
            companyName = "AmadoMuerte"
            fileDescription = "Waxlight Launcher"
            fileVersion = $WindowsVersion
            internalName = "waxlight"
            legalCopyright = $ExpectedCopyright
            originalFileName = "waxlight.exe"
            productName = "Waxlight Launcher"
            productVersion = $WindowsVersion
        }
    }

    $DefinitionJson = $Definition | ConvertTo-Json -Depth 20
    [System.IO.File]::WriteAllText(
        $DefinitionPath,
        $DefinitionJson + [Environment]::NewLine,
        [System.Text.UTF8Encoding]::new($false)
    )

    Write-Host "Patching Windows VersionInfo before NSIS packaging..."
    Write-Host "  Executable: $ResolvedExecutable"
    Write-Host "  Version:    $WindowsVersion"
    Write-Host "  Tool:       $($ResEdit.Source)"

    & $ResEdit.Source `
        --in $ResolvedExecutable `
        --out $TemporaryExecutable `
        --definition $DefinitionPath

    if ($LASTEXITCODE -ne 0) {
        throw "resedit-cli failed with exit code $LASTEXITCODE"
    }

    if (-not (Test-Path -LiteralPath $TemporaryExecutable -PathType Leaf)) {
        throw "resedit-cli did not create the patched executable"
    }
    if ((Get-Item -LiteralPath $TemporaryExecutable).Length -le 0) {
        throw "The patched executable is empty"
    }

    $VersionInfo = (Get-Item -LiteralPath $TemporaryExecutable).VersionInfo
    $ValidationErrors = [System.Collections.Generic.List[string]]::new()

    Test-MetadataValue $ValidationErrors "FileDescription" `
        $VersionInfo.FileDescription "Waxlight Launcher"
    Test-MetadataValue $ValidationErrors "ProductName" `
        $VersionInfo.ProductName "Waxlight Launcher"
    Test-MetadataValue $ValidationErrors "CompanyName" `
        $VersionInfo.CompanyName "AmadoMuerte"
    Test-MetadataValue $ValidationErrors "LegalCopyright" `
        $VersionInfo.LegalCopyright $ExpectedCopyright
    Test-MetadataValue $ValidationErrors "FileVersion" `
        $VersionInfo.FileVersion $WindowsVersion
    Test-MetadataValue $ValidationErrors "ProductVersion" `
        $VersionInfo.ProductVersion $WindowsVersion
    Test-MetadataValue $ValidationErrors "Comments" `
        $VersionInfo.Comments $ExpectedComments

    if ($ValidationErrors.Count -gt 0) {
        foreach ($ValidationError in $ValidationErrors) {
            Write-Host "  - $ValidationError" -ForegroundColor Red
        }
        throw "Patched Windows metadata validation failed"
    }

    # The temporary file is created next to the executable so File.Replace is
    # atomic and preserves the destination file's ACL on the same volume.
    [System.IO.File]::Replace(
        $TemporaryExecutable,
        $ResolvedExecutable,
        $null
    )

    Write-Host "Windows VersionInfo patched and verified" -ForegroundColor Green
}
finally {
    if (Test-Path -LiteralPath $TemporaryExecutable) {
        Remove-Item -LiteralPath $TemporaryExecutable -Force
    }
    if (Test-Path -LiteralPath $TemporaryDirectory) {
        Remove-Item -LiteralPath $TemporaryDirectory -Recurse -Force
    }
}
