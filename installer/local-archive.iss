; Inno Setup Script for Local Archive (الأرشيف المحلي)
; Compiles a standard professional Windows installer

#define MyAppName "الأرشيف المحلي"
#define MyAppNameEn "Local Archive"
#define MyAppVersion "1.1.0"
#define MyAppPublisher "صادق العبودي"
#define MyAppExeName "archive.exe"

[Setup]
AppId={{D1A39F7C-9042-4F11-827B-1CE3894E7A22}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\LocalArchive
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=.
OutputBaseFilename=LocalArchive-Installer
SetupIconFile=..\static\favicon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest

[Languages]
Name: "arabic"; MessagesFile: "compiler:Languages\Arabic.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "startupicon"; Description: "تشغيل التطبيق تلقائياً عند بدء تشغيل ويندوز (Autostart on boot)"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "..\archive.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\static\favicon.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\favicon.ico"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon; IconFilename: "{app}\favicon.ico"
Name: "{userstartup}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Parameters: "-no-browser"; Tasks: startupicon; IconFilename: "{app}\favicon.ico"

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent
