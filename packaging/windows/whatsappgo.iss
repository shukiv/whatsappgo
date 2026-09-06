; Inno Setup script for WhatsAppGo. packaging\windows\build.ps1 passes StageDir
; and OutputDir; building it by hand needs both defined on the iscc command line.
#ifndef StageDir
  #error Define StageDir, the folder windeployqt was run against.
#endif
#ifndef OutputDir
  #define OutputDir "."
#endif

[Setup]
AppId={{7F2B4E4B-9E2E-4D0F-9C3B-2C0B9F1D6A11}
AppName=WhatsAppGo
AppVersion=0.1.5
AppPublisher=WhatsAppGo
DefaultDirName={autopf}\WhatsAppGo
DefaultGroupName=WhatsAppGo
UninstallDisplayIcon={app}\whatsappgo.exe
OutputDir={#OutputDir}
OutputBaseFilename=WhatsAppGo-Setup
Compression=lzma2
SolidCompression=yes
; Per-user installs need no administrator, which is what most people want for a
; messaging client, and it keeps the profile directories under the same account.
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesInstallIn64BitMode=x64compatible

[Files]
Source: "{#StageDir}\*"; DestDir: "{app}"; Flags: recursesubdirs createallsubdirs ignoreversion

[Icons]
Name: "{group}\WhatsAppGo"; Filename: "{app}\whatsappgo.exe"
Name: "{autodesktop}\WhatsAppGo"; Filename: "{app}\whatsappgo.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"

[Run]
Filename: "{app}\whatsappgo.exe"; Description: "Start WhatsAppGo"; Flags: nowait postinstall skipifsilent

; The daemon keeps running after the window closes. Stop it before replacing
; the files, or the uninstaller leaves a process holding the databases open.
[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM whatsappd.exe"; Flags: runhidden; RunOnceId: "StopDaemon"
