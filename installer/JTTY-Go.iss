#define AppName "JTTY-Go"
#define AppVersion "5.36.14"
#define AppPublisher "BH2VSQ"
#define AppExeName "JTTY-Go.exe"

#ifndef SourceExe
  #define SourceExe "..\build\bin\JTTY-Go.exe"
#endif

[Setup]
AppId={{B6E5A0E2-8F0A-4C9A-9A1E-6F4C4F5A7A36}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher={#AppPublisher}
DefaultDirName={localappdata}\Programs\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=no
PrivilegesRequired=lowest
OutputDir=..\dist\windows\installer
OutputBaseFilename=JTTY-Go-{#AppVersion}-Setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\{#AppExeName}
LicenseFile=..\LICENSE
CloseApplications=yes
RestartApplications=no
ChangesAssociations=no

[Files]
; JTTY-Go.exe contains the frontend and embedded Hamlib runtime.
Source: "{#SourceExe}"; DestDir: "{app}"; Flags: ignoreversion restartreplace
Source: "..\licenses\WSJT-X-GPL-3.0.txt"; DestDir: "{app}\licenses"; Flags: ignoreversion
Source: "..\licenses\JTTY-PORT-NOTICE.md"; DestDir: "{app}\licenses"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"

[Run]
Filename: "{app}\{#AppExeName}"; Description: "Launch {#AppName}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; The application directory contains only installed/runtime files.
; User data remains in %APPDATA%\JTTY-Go and is intentionally preserved.
Type: filesandordirs; Name: "{app}"
