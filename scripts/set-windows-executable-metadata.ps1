#Requires -Version 5.1

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

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
$BackupExecutable = Join-Path `
    $ExecutableDirectory `
    ("." + $ExecutableName + ".metadata-backup-" + [Guid]::NewGuid().ToString("N") + ".exe")
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

    Write-Host "Patched Windows executable metadata:"
    Write-Host "  FileDescription: $($VersionInfo.FileDescription)"
    Write-Host "  ProductName:     $($VersionInfo.ProductName)"
    Write-Host "  CompanyName:     $($VersionInfo.CompanyName)"
    Write-Host "  LegalCopyright:  $($VersionInfo.LegalCopyright)"
    Write-Host "  FileVersion:     $($VersionInfo.FileVersion)"
    Write-Host "  ProductVersion:  $($VersionInfo.ProductVersion)"
    Write-Host "  Comments:        $($VersionInfo.Comments)"

    # Do not pass an empty collection into a mandatory PowerShell parameter.
    # Windows PowerShell 5.1 rejects that before the validation function runs.
    # Validate the VersionInfo properties directly instead.
    $ExpectedMetadata = [ordered]@{
        FileDescription = "Waxlight Launcher"
        ProductName = "Waxlight Launcher"
        CompanyName = "AmadoMuerte"
        LegalCopyright = $ExpectedCopyright
        FileVersion = $WindowsVersion
        ProductVersion = $WindowsVersion
        Comments = $ExpectedComments
    }
    $ValidationErrors = @()

    foreach ($Entry in $ExpectedMetadata.GetEnumerator()) {
        $Property = $VersionInfo.PSObject.Properties[$Entry.Key]
        $ActualValue = if ($null -eq $Property) {
            $null
        }
        else {
            [string]$Property.Value
        }
        $ExpectedValue = [string]$Entry.Value

        if ($ActualValue -ne $ExpectedValue) {
            $ValidationErrors +=
                "$($Entry.Key) must be '$ExpectedValue', got '$ActualValue'"
        }
    }

    if ($ValidationErrors.Count -gt 0) {
        foreach ($ValidationError in $ValidationErrors) {
            Write-Host "  - $ValidationError" -ForegroundColor Red
        }
        throw "Patched Windows metadata validation failed"
    }

    # Windows PowerShell 5.1 can bind $null as an empty backup path when calling
    # File.Replace, which causes "The path is not of a legal form." Use a real
    # backup path, then remove it after the final executable has been verified.
    try {
        [System.IO.File]::Replace(
            $TemporaryExecutable,
            $ResolvedExecutable,
            $BackupExecutable,
            $true
        )

        $FinalVersionInfo = (Get-Item -LiteralPath $ResolvedExecutable).VersionInfo
        $FinalValidationErrors = @()

        foreach ($Entry in $ExpectedMetadata.GetEnumerator()) {
            $Property = $FinalVersionInfo.PSObject.Properties[$Entry.Key]
            $ActualValue = if ($null -eq $Property) {
                $null
            }
            else {
                [string]$Property.Value
            }
            $ExpectedValue = [string]$Entry.Value

            if ($ActualValue -ne $ExpectedValue) {
                $FinalValidationErrors +=
                    "$($Entry.Key) must be '$ExpectedValue', got '$ActualValue'"
            }
        }

        if ($FinalValidationErrors.Count -gt 0) {
            foreach ($ValidationError in $FinalValidationErrors) {
                Write-Host "  - $ValidationError" -ForegroundColor Red
            }
            throw "Final Windows metadata validation failed after replacing the executable"
        }
    }
    catch {
        if (Test-Path -LiteralPath $BackupExecutable -PathType Leaf) {
            [System.IO.File]::Copy(
                $BackupExecutable,
                $ResolvedExecutable,
                $true
            )
        }
        throw
    }

    Write-Host "Windows VersionInfo patched and verified" -ForegroundColor Green
}
finally {
    if (Test-Path -LiteralPath $TemporaryExecutable) {
        Remove-Item -LiteralPath $TemporaryExecutable -Force
    }
    if (Test-Path -LiteralPath $BackupExecutable) {
        Remove-Item -LiteralPath $BackupExecutable -Force
    }
    if (Test-Path -LiteralPath $TemporaryDirectory) {
        Remove-Item -LiteralPath $TemporaryDirectory -Recurse -Force
    }
}
