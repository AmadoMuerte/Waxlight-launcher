Unicode true

####
## Waxlight Launcher NSIS installer.
## Keep this file under packaging/windows because build/ is generated output.
####

!include "wails_tools.nsh"

# Windows VERSIONINFO requires four numeric components. build-windows.ps1
# temporarily supplies that numeric value through wails.json.
VIProductVersion "${INFO_PRODUCTVERSION}"
VIFileVersion "${INFO_PRODUCTVERSION}"
VIAddVersionKey "CompanyName" "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright" "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName" "${INFO_PRODUCTNAME}"

ManifestDPIAware true
RequestExecutionLevel user

!include "MUI.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING

Var TelemetryCheckbox
Var TelemetryOptIn

!insertmacro MUI_PAGE_WELCOME
# SignPath Foundation requires software that transfers user data to display its
# privacy policy during installation. build-windows.ps1 stages this file beside
# project.nsi before Wails invokes NSIS.
!insertmacro MUI_PAGE_LICENSE "PRIVACY.md"
!insertmacro MUI_PAGE_DIRECTORY
Page custom TelemetryPageCreate TelemetryPageLeave
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"

# A per-user installer keeps the privacy choice in the same user profile that
# starts Waxlight, including when the computer has a separate administrator.
InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"

ShowInstDetails show

Function .onInit
    !insertmacro wails.checkArchitecture
    # Telemetry is opt-in. Silent installs and upgrades keep this disabled here;
    # an existing in-app preference is preserved by the launcher backend.
    StrCpy $TelemetryOptIn "0"
FunctionEnd

Function TelemetryPageCreate
    nsDialogs::Create 1018
    Pop $0
    ${If} $0 == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 14u "Privacy & telemetry"
    Pop $0
    ${NSD_CreateLabel} 0 20u 100% 42u "Waxlight can send limited usage telemetry to help improve reliability. Telemetry is optional and is disabled by default. You can change this later in Settings → Privacy & telemetry."
    Pop $0
    ${NSD_CreateCheckbox} 0 70u 100% 14u "Enable optional usage telemetry"
    Pop $TelemetryCheckbox
    ${NSD_Uncheck} $TelemetryCheckbox

    nsDialogs::Show
FunctionEnd

Function TelemetryPageLeave
    ${NSD_GetState} $TelemetryCheckbox $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $TelemetryOptIn "1"
    ${Else}
        StrCpy $TelemetryOptIn "0"
    ${EndIf}
FunctionEnd

Section
    !insertmacro wails.setShellContext
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files

    # Persist only an explicit installer opt-in. The launcher consumes this
    # one-time marker on first startup and never lets it override existing
    # settings during upgrades. Silent installs do not create a marker.
    IfSilent telemetry_marker_done
    CreateDirectory "$APPDATA\waxlight"
    ${If} $TelemetryOptIn == "1"
        FileOpen $0 "$APPDATA\waxlight\installer-telemetry-opt-in" w
        FileWrite $0 "1"
        FileClose $0
    ${Else}
        Delete "$APPDATA\waxlight\installer-telemetry-opt-in"
    ${EndIf}
telemetry_marker_done:

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols
    !insertmacro wails.deleteUninstaller
SectionEnd
