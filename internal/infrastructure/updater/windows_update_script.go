package updater

// windowsUpdateHelperScript is launched elevated before the Wails process exits.
// It waits for the current process, installs the verified NSIS package, and then
// starts the newly installed launcher through Explorer so it does not inherit
// the administrator token.
const windowsUpdateHelperScript = `param(
    [Parameter(Mandatory = $true)]
    [string]$InstallerPath,

    [Parameter(Mandatory = $true)]
    [int]$CurrentPID,

    [Parameter(Mandatory = $true)]
    [string]$CurrentExecutable
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$LogDirectory = Join-Path $env:LOCALAPPDATA "Waxlight Launcher"
$LogPath = Join-Path $LogDirectory "update.log"
New-Item -ItemType Directory -Path $LogDirectory -Force | Out-Null

function Write-UpdateLog {
    param([string]$Message)
    Add-Content -LiteralPath $LogPath -Value ("[{0}] {1}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Message)
}

try {
    Write-UpdateLog "Waiting for launcher process $CurrentPID to exit"
    $Process = Get-Process -Id $CurrentPID -ErrorAction SilentlyContinue
    if ($null -ne $Process) {
        $Process | Wait-Process -ErrorAction Stop
    }
    Start-Sleep -Milliseconds 500

    if (-not (Test-Path -LiteralPath $InstallerPath -PathType Leaf)) {
        throw "Downloaded installer no longer exists: $InstallerPath"
    }

    Write-UpdateLog "Starting installer: $InstallerPath"
    $Installer = Start-Process -FilePath $InstallerPath -ArgumentList @("/S") -WorkingDirectory (Split-Path -Parent $InstallerPath) -PassThru -Wait

    if ($Installer.ExitCode -ne 0) {
        throw "Installer exited with code $($Installer.ExitCode)"
    }

    $ProgramFiles64 = $env:ProgramW6432
    if ([string]::IsNullOrWhiteSpace($ProgramFiles64)) {
        $ProgramFiles64 = $env:ProgramFiles
    }

    $Candidates = @(
        (Join-Path $ProgramFiles64 "Waxlight Launcher\waxlight.exe"),
        (Join-Path $env:ProgramFiles "Waxlight Launcher\waxlight.exe")
    ) | Select-Object -Unique

    $InstalledExecutable = $null
    for ($Attempt = 0; $Attempt -lt 30 -and $null -eq $InstalledExecutable; $Attempt++) {
        foreach ($Candidate in $Candidates) {
            if (-not [string]::IsNullOrWhiteSpace($Candidate) -and (Test-Path -LiteralPath $Candidate -PathType Leaf)) {
                $InstalledExecutable = $Candidate
                break
            }
        }
        if ($null -eq $InstalledExecutable) {
            Start-Sleep -Milliseconds 500
        }
    }

    if ($null -eq $InstalledExecutable) {
        throw "The updated launcher executable was not found in the clean installation directory"
    }

    $InstalledDirectory = Split-Path -Parent $InstalledExecutable
    $LegacyDirectory = Split-Path -Parent $CurrentExecutable
    $LegacySuffix = [System.IO.Path]::Combine("AmadoMuerte", "Waxlight Launcher")
    if (
        -not [string]::Equals($LegacyDirectory, $InstalledDirectory, [System.StringComparison]::OrdinalIgnoreCase) -and
        $LegacyDirectory.EndsWith($LegacySuffix, [System.StringComparison]::OrdinalIgnoreCase)
    ) {
        try {
            Remove-Item -LiteralPath $LegacyDirectory -Recurse -Force -ErrorAction Stop
            $LegacyPublisherDirectory = Split-Path -Parent $LegacyDirectory
            Remove-Item -LiteralPath $LegacyPublisherDirectory -Force -ErrorAction SilentlyContinue
            Write-UpdateLog "Removed legacy installation directory: $LegacyDirectory"
        }
        catch {
            Write-UpdateLog ("Legacy installation cleanup was skipped: " + $_.Exception.Message)
        }
    }

    Write-UpdateLog "Starting updated launcher: $InstalledExecutable"
    try {
        $Shell = New-Object -ComObject Shell.Application
        $Shell.ShellExecute($InstalledExecutable, "", $InstalledDirectory, "open", 1)
    }
    catch {
        Start-Process -FilePath $InstalledExecutable -WorkingDirectory $InstalledDirectory
    }

    Write-UpdateLog "Update completed successfully"
    exit 0
}
catch {
    Write-UpdateLog ("Update failed: " + $_.Exception.Message)
    exit 1
}
`
