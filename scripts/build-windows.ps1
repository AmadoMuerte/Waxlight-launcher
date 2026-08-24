#Requires -Version 5.1

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [Parameter(Mandatory = $false)]
    [string]$OutputDirectory = "release"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Invoke-CheckedCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Description,

        [Parameter(Mandatory = $true)]
        [scriptblock]$Command
    )

    Write-Host ""
    Write-Host $Description
    Write-Host ("-" * $Description.Length)

    & $Command

    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE"
    }
}

function Get-ReleaseVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Value
    )

    $Normalized = $Value.Trim()

    if ($Normalized.StartsWith("v")) {
        $Normalized = $Normalized.Substring(1)
    }

    if ($Normalized -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$') {
        throw "Invalid semantic version: $Value"
    }

    return $Normalized
}

function Get-WindowsFileVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ReleaseVersion
    )

    $BaseVersion = ($ReleaseVersion -split "-", 2)[0]
    $Parts = $BaseVersion -split "\."

    if ($Parts.Count -ne 3) {
        throw "Unable to convert '$ReleaseVersion' to a Windows file version"
    }

    $BuildNumber = 0

    if ($ReleaseVersion -match "-") {
        $Prerelease = ($ReleaseVersion -split "-", 2)[1]
        $PrereleaseMatch = [regex]::Match($Prerelease, '^(alpha|beta|rc)\.([1-9][0-9]{0,3})$')
        if (-not $PrereleaseMatch.Success) {
            throw "Windows prereleases must use alpha.N, beta.N, or rc.N: $ReleaseVersion"
        }

        $Sequence = [int]$PrereleaseMatch.Groups[2].Value
        if ($Sequence -gt 19999) {
            throw "Windows prerelease sequence must not exceed 19999: $ReleaseVersion"
        }

        # Windows has one 16-bit revision field. Reserve disjoint ranges so
        # alpha.1, beta.1, and rc.1 cannot collapse to the same file version.
        $BuildNumber = switch ($PrereleaseMatch.Groups[1].Value) {
            "alpha" { $Sequence }
            "beta" { 20000 + $Sequence }
            "rc" { 40000 + $Sequence }
        }
    }

    return "$($Parts[0]).$($Parts[1]).$($Parts[2]).$BuildNumber"
}

function Set-WailsProductVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        [string]$ProductVersion
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Wails configuration file not found: $Path"
    }

    $Config = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    $CopyrightSymbol = [char]0x00A9
    $LegalCopyright = "Copyright $CopyrightSymbol 2026 AmadoMuerte"

    if ($null -eq $Config.info) {
        $Config | Add-Member `
            -MemberType NoteProperty `
            -Name "info" `
            -Value ([PSCustomObject]@{})
    }

    $Metadata = [ordered]@{
        companyName = "AmadoMuerte"
        productName = "Waxlight Launcher"
        productVersion = $ProductVersion
        copyright = $LegalCopyright
        comments = "Free software licensed under the GNU General Public License v3.0."
    }

    foreach ($Entry in $Metadata.GetEnumerator()) {
        if ($null -eq $Config.info.PSObject.Properties[$Entry.Key]) {
            $Config.info | Add-Member `
                -MemberType NoteProperty `
                -Name $Entry.Key `
                -Value $Entry.Value
        }
        else {
            $Config.info.PSObject.Properties[$Entry.Key].Value = $Entry.Value
        }
    }

    # Wails normally links VersionInfo through a generated .syso file. The
    # project needs CGO for go-sqlite3, so the final executable is linked by
    # MinGW. Patch the already-built executable in a Wails post-build hook,
    # before Wails creates the NSIS installer. This makes the standalone EXE
    # and the EXE inside the installer use the exact same verified metadata.
    $WindowsPostBuildHook =
        'powershell.exe -NoProfile -ExecutionPolicy Bypass -File ../../scripts/set-windows-executable-metadata.ps1 -ExecutablePath ${bin}'

    if ($null -eq $Config.PSObject.Properties["postBuildHooks"]) {
        $Config | Add-Member `
            -MemberType NoteProperty `
            -Name "postBuildHooks" `
            -Value ([PSCustomObject]@{})
    }

    if ($null -eq $Config.postBuildHooks.PSObject.Properties["windows/*"]) {
        $Config.postBuildHooks | Add-Member `
            -MemberType NoteProperty `
            -Name "windows/*" `
            -Value $WindowsPostBuildHook
    }
    else {
        $Config.postBuildHooks.PSObject.Properties["windows/*"].Value =
            $WindowsPostBuildHook
    }

    $Json = $Config | ConvertTo-Json -Depth 100

    [System.IO.File]::WriteAllText(
        $Path,
        $Json + [Environment]::NewLine,
        [System.Text.UTF8Encoding]::new($false)
    )
}

function New-WindowsInfoJson {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $Directory = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $Directory)) {
        New-Item -ItemType Directory -Path $Directory -Force | Out-Null
    }

    # Wails v2 does not read a flat object here. It expects its own template
    # schema with `fixed` and `info` sections, then substitutes values from the
    # active wails.json `info` block before generating the Windows .syso file.
    $Info = [ordered]@{
        fixed = [ordered]@{
            file_version = "{{.Info.ProductVersion}}"
        }
        info = [ordered]@{
            "0000" = [ordered]@{
                ProductVersion = "{{.Info.ProductVersion}}"
                CompanyName = "{{.Info.CompanyName}}"
                FileDescription = "{{.Info.ProductName}}"
                LegalCopyright = "{{.Info.Copyright}}"
                ProductName = "{{.Info.ProductName}}"
                Comments = "{{.Info.Comments}}"
            }
        }
    }

    $Json = $Info | ConvertTo-Json -Depth 100
    [System.IO.File]::WriteAllText(
        $Path,
        $Json + [Environment]::NewLine,
        [System.Text.UTF8Encoding]::new($false)
    )

    $GeneratedInfo = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    if (
        $null -eq $GeneratedInfo.fixed -or
        $null -eq $GeneratedInfo.fixed.file_version -or
        $null -eq $GeneratedInfo.info -or
        $null -eq $GeneratedInfo.info.PSObject.Properties["0000"]
    ) {
        throw "Generated Windows info.json does not match the Wails v2 schema"
    }

    Write-Host "Generated Wails v2 Windows info.json template: $Path"
}

function Test-WindowsMetadata {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ExecutablePath,

        [Parameter(Mandatory = $true)]
        [string]$ExpectedFileVersion,

        [Parameter(Mandatory = $false)]
        [string]$ExpectedFileDescription = "Waxlight Launcher"
    )

    if (-not (Test-Path -LiteralPath $ExecutablePath)) {
        throw "Executable not found for metadata validation: $ExecutablePath"
    }

    $VersionInfo = (Get-Item -LiteralPath $ExecutablePath).VersionInfo

    Write-Host ""
    Write-Host "Windows executable metadata:"
    Write-Host "  FileDescription: $($VersionInfo.FileDescription)"
    Write-Host "  ProductName:     $($VersionInfo.ProductName)"
    Write-Host "  CompanyName:     $($VersionInfo.CompanyName)"
    Write-Host "  LegalCopyright:  $($VersionInfo.LegalCopyright)"
    Write-Host "  FileVersion:     $($VersionInfo.FileVersion)"
    Write-Host "  ProductVersion:  $($VersionInfo.ProductVersion)"
    Write-Host "  Comments:        $($VersionInfo.Comments)"
    Write-Host ""

    $CopyrightSymbol = [char]0x00A9
    $ExpectedCopyright = "Copyright $CopyrightSymbol 2026 AmadoMuerte"

    $Errors = @()

    if ($VersionInfo.FileDescription -ne $ExpectedFileDescription) {
        $Errors += "FileDescription must be '$ExpectedFileDescription', got '$($VersionInfo.FileDescription)'"
    }

    if ($VersionInfo.ProductName -ne "Waxlight Launcher") {
        $Errors += "ProductName must be 'Waxlight Launcher', got '$($VersionInfo.ProductName)'"
    }

    if ($VersionInfo.CompanyName -ne "AmadoMuerte") {
        $Errors += "CompanyName must be 'AmadoMuerte', got '$($VersionInfo.CompanyName)'"
    }

    if ($VersionInfo.LegalCopyright -ne $ExpectedCopyright) {
        $Errors += "LegalCopyright must be '$ExpectedCopyright', got '$($VersionInfo.LegalCopyright)'"
    }

    if ([string]::IsNullOrEmpty($VersionInfo.CompanyName)) {
        $Errors += "CompanyName must not be empty"
    }

    if ([string]::IsNullOrEmpty($VersionInfo.ProductName)) {
        $Errors += "ProductName must not be empty"
    }

    if ($VersionInfo.FileVersion -ne $ExpectedFileVersion) {
        $Errors += "FileVersion must be '$ExpectedFileVersion', got '$($VersionInfo.FileVersion)'"
    }

    if ($VersionInfo.ProductVersion -ne $ExpectedFileVersion) {
        $Errors += "ProductVersion must be '$ExpectedFileVersion', got '$($VersionInfo.ProductVersion)'"
    }

    if ($Errors.Count -gt 0) {
        Write-Host "Windows metadata validation failed:" -ForegroundColor Red
        foreach ($Error in $Errors) {
            Write-Host "  - $Error" -ForegroundColor Red
        }
        throw "Windows metadata validation failed with $($Errors.Count) error(s)"
    }

    Write-Host "Windows metadata validation passed" -ForegroundColor Green
}

function Copy-ReleaseAsset {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Source,

        [Parameter(Mandatory = $true)]
        [string]$Destination
    )

    if (-not (Test-Path -LiteralPath $Source)) {
        throw "Release asset not found: $Source"
    }

    Copy-Item `
        -LiteralPath $Source `
        -Destination $Destination `
        -Force

    if ((Get-Item -LiteralPath $Destination).Length -le 0) {
        throw "Created release asset is empty: $Destination"
    }
}

$ReleaseVersion = Get-ReleaseVersion -Value $Version
$WindowsFileVersion = Get-WindowsFileVersion -ReleaseVersion $ReleaseVersion

$RepositoryRoot = Split-Path -Parent $PSScriptRoot
$WailsProjectDirectory = Join-Path $RepositoryRoot "cmd\waxlight"
$BuildDirectory = Join-Path $RepositoryRoot "build"
$BuildBinDirectory = Join-Path $BuildDirectory "bin"
$FrontendDirectory = Join-Path $RepositoryRoot "frontend"
$FrontendEntry = Join-Path $FrontendDirectory "dist\index.html"
$ApplicationIconPng = Join-Path $WailsProjectDirectory "appicon.png"
$ApplicationIconIco = Join-Path $WailsProjectDirectory "appicon.ico"
$WailsAppIcon = Join-Path $BuildDirectory "appicon.png"
$WailsWindowsDirectory = Join-Path $BuildDirectory "windows"
$WailsWindowsIcon = Join-Path $WailsWindowsDirectory "icon.ico"
$WailsWindowsInfo = Join-Path $WailsWindowsDirectory "info.json"
$InstallerTemplateSource = Join-Path $RepositoryRoot "packaging\windows\project.nsi"
$InstallerTemplateDirectory = Join-Path $WailsWindowsDirectory "installer"
$InstallerTemplateDestination = Join-Path $InstallerTemplateDirectory "project.nsi"
$PrivacyPolicySource = Join-Path $RepositoryRoot "docs\PRIVACY.md"
$PrivacyPolicyDestination = Join-Path $InstallerTemplateDirectory "PRIVACY.md"
$LicenseFile = Join-Path $RepositoryRoot "LICENSE"
$NoticeFile = Join-Path $RepositoryRoot "NOTICE"

if ([System.IO.Path]::IsPathRooted($OutputDirectory)) {
    $ResolvedOutputDirectory = $OutputDirectory
}
else {
    $ResolvedOutputDirectory = Join-Path $RepositoryRoot $OutputDirectory
}

$RootWailsConfig = Join-Path $RepositoryRoot "wails.json"
$ProjectWailsConfig = Join-Path $WailsProjectDirectory "wails.json"

$ConfigBackups = @{}

Write-Host "Waxlight release version: $ReleaseVersion"
Write-Host "Windows file version:    $WindowsFileVersion"
Write-Host "Output directory:        $ResolvedOutputDirectory"

New-Item `
    -ItemType Directory `
    -Path $ResolvedOutputDirectory `
    -Force | Out-Null

foreach ($RequiredAsset in @($ApplicationIconPng, $ApplicationIconIco, $LicenseFile, $NoticeFile, $InstallerTemplateSource, $PrivacyPolicySource)) {
    if (-not (Test-Path -LiteralPath $RequiredAsset -PathType Leaf)) {
        throw "Required asset is missing: $RequiredAsset"
    }

    if ((Get-Item -LiteralPath $RequiredAsset).Length -le 0) {
        throw "Required asset is empty: $RequiredAsset"
    }
}

# Clean only generated binaries. Wails -clean would delete the custom icon
# files staged under build/ before they are bundled into the executable.
if (Test-Path -LiteralPath $BuildBinDirectory) {
    Remove-Item `
        -LiteralPath $BuildBinDirectory `
        -Recurse `
        -Force
}

New-Item `
    -ItemType Directory `
    -Path $BuildBinDirectory `
    -Force | Out-Null

New-Item `
    -ItemType Directory `
    -Path $WailsWindowsDirectory `
    -Force | Out-Null

# Wails reads build/appicon.png and build/windows/icon.ico when producing the
# executable and NSIS installer. The tracked source files live beside main.go;
# build/ remains generated output.
Copy-Item `
    -LiteralPath $ApplicationIconPng `
    -Destination $WailsAppIcon `
    -Force

Copy-Item `
    -LiteralPath $ApplicationIconIco `
    -Destination $WailsWindowsIcon `
    -Force

# Generate Windows info.json for VersionInfo metadata
New-WindowsInfoJson `
    -Path $WailsWindowsInfo

New-Item `
    -ItemType Directory `
    -Path $InstallerTemplateDirectory `
    -Force | Out-Null

Copy-Item `
    -LiteralPath $InstallerTemplateSource `
    -Destination $InstallerTemplateDestination `
    -Force

Copy-Item `
    -LiteralPath $PrivacyPolicySource `
    -Destination $PrivacyPolicyDestination `
    -Force

foreach ($ConfigPath in @($RootWailsConfig, $ProjectWailsConfig)) {
    if (Test-Path -LiteralPath $ConfigPath) {
        $ConfigBackups[$ConfigPath] = Get-Content -LiteralPath $ConfigPath -Raw
        Set-WailsProductVersion `
            -Path $ConfigPath `
            -ProductVersion $WindowsFileVersion
    }
}

try {
    Invoke-CheckedCommand `
        -Description "Installing frontend dependencies..." `
        -Command {
            npm ci `
                --include=dev `
                --prefix $FrontendDirectory
        }

    Invoke-CheckedCommand `
        -Description "Building frontend assets..." `
        -Command {
            npm `
                --prefix $FrontendDirectory `
                run build
        }

    if (-not (Test-Path -LiteralPath $FrontendEntry)) {
        throw "Frontend build did not create: $FrontendEntry"
    }

    if ((Get-Item -LiteralPath $FrontendEntry).Length -le 0) {
        throw "Frontend entry file is empty: $FrontendEntry"
    }

    Push-Location $WailsProjectDirectory

    try {
        Invoke-CheckedCommand `
            -Description "Building Windows application and NSIS installer..." `
            -Command {
                wails build `
                    -skipbindings `
                    -platform windows/amd64 `
                    -nsis `
                    -trimpath `
                    -ldflags="-s -w -X github.com/AmadoMuerte/Waxlight-launcher/internal/version.buildVersion=$ReleaseVersion"
            }
    }
    finally {
        Pop-Location
    }

    $ApplicationExecutable = Join-Path $BuildBinDirectory "waxlight.exe"

    if (-not (Test-Path -LiteralPath $ApplicationExecutable)) {
        $ApplicationExecutable = Get-ChildItem `
            -Path $BuildBinDirectory `
            -Filter "*.exe" `
            -File `
            -Recurse |
            Where-Object {
                $_.Name -notmatch "installer|setup|uninstall"
            } |
            Select-Object -First 1 |
            ForEach-Object FullName
    }

    if (-not $ApplicationExecutable) {
        throw "Wails application executable was not found in $BuildBinDirectory"
    }

    $InstallerExecutable = Get-ChildItem `
        -Path $BuildBinDirectory `
        -Filter "*.exe" `
        -File `
        -Recurse |
        Where-Object {
            $_.Name -match "installer|setup"
        } |
        Select-Object -First 1 |
        ForEach-Object FullName

    if (-not $InstallerExecutable) {
        Write-Host "Files found in ${BuildBinDirectory}:"
        Get-ChildItem -Path $BuildBinDirectory -File -Recurse |
            Format-Table FullName, Length

        throw "NSIS installer was not found in $BuildBinDirectory"
    }

    Test-WindowsMetadata `
        -ExecutablePath $ApplicationExecutable `
        -ExpectedFileVersion $WindowsFileVersion

    Test-WindowsMetadata `
        -ExecutablePath $InstallerExecutable `
        -ExpectedFileVersion $WindowsFileVersion `
        -ExpectedFileDescription "Waxlight Launcher Installer"

    $StandaloneName =
        "Waxlight-Launcher-v$ReleaseVersion-windows-amd64.exe"

    $PortableName =
        "Waxlight-Launcher-v$ReleaseVersion-windows-amd64-portable.zip"

    $InstallerName =
        "Waxlight-Launcher-v$ReleaseVersion-windows-amd64-installer.exe"

    $StandalonePath = Join-Path $ResolvedOutputDirectory $StandaloneName
    $PortablePath = Join-Path $ResolvedOutputDirectory $PortableName
    $InstallerPath = Join-Path $ResolvedOutputDirectory $InstallerName

    Copy-ReleaseAsset `
        -Source $ApplicationExecutable `
        -Destination $StandalonePath

    Copy-ReleaseAsset `
        -Source $InstallerExecutable `
        -Destination $InstallerPath

    $PortableStagingDirectory = Join-Path `
        ([System.IO.Path]::GetTempPath()) `
        ("waxlight-portable-" + [Guid]::NewGuid().ToString("N"))

    try {
        New-Item `
            -ItemType Directory `
            -Path $PortableStagingDirectory `
            -Force | Out-Null

        Copy-Item `
            -LiteralPath $ApplicationExecutable `
            -Destination (Join-Path $PortableStagingDirectory "waxlight.exe") `
            -Force

        Copy-Item `
            -LiteralPath $LicenseFile `
            -Destination (Join-Path $PortableStagingDirectory "LICENSE") `
            -Force

        Copy-Item `
            -LiteralPath $NoticeFile `
            -Destination (Join-Path $PortableStagingDirectory "NOTICE") `
            -Force

        if (Test-Path -LiteralPath $PortablePath) {
            Remove-Item -LiteralPath $PortablePath -Force
        }

        Compress-Archive `
            -Path (Join-Path $PortableStagingDirectory "*") `
            -DestinationPath $PortablePath `
            -CompressionLevel Optimal

        $PortableVerificationDirectory = Join-Path $PortableStagingDirectory "verify"
        Expand-Archive `
            -LiteralPath $PortablePath `
            -DestinationPath $PortableVerificationDirectory `
            -Force
        Test-WindowsMetadata `
            -ExecutablePath (Join-Path $PortableVerificationDirectory "waxlight.exe") `
            -ExpectedFileVersion $WindowsFileVersion
    }
    finally {
        if (Test-Path -LiteralPath $PortableStagingDirectory) {
            Remove-Item `
                -LiteralPath $PortableStagingDirectory `
                -Recurse `
                -Force
        }
    }

    $ExpectedAssets = @(
        $StandalonePath,
        $PortablePath,
        $InstallerPath
    )

    foreach ($Asset in $ExpectedAssets) {
        if (-not (Test-Path -LiteralPath $Asset)) {
            throw "Missing Windows release asset: $Asset"
        }

        if ((Get-Item -LiteralPath $Asset).Length -le 0) {
            throw "Empty Windows release asset: $Asset"
        }
    }

    Write-Host ""
    Write-Host "Windows release assets:"
    Get-Item -LiteralPath $ExpectedAssets |
        Format-Table Name, Length
}
finally {
    foreach ($Entry in $ConfigBackups.GetEnumerator()) {
        [System.IO.File]::WriteAllText(
            $Entry.Key,
            $Entry.Value,
            [System.Text.UTF8Encoding]::new($false)
        )
    }
}
