; ============================================================================
; Hifeh Tasty Kitchen — Inno Setup Installer Script
; ============================================================================
; How to compile:
;   1. Install Inno Setup 6 from https://jrsoftware.org/isinfo.php
;   2. Place this file alongside: posd.exe, logo.png, README.md
;      (or adjust paths in [Files] below)
;   3. Open this file in Inno Setup Compiler → Build → Compile
;   4. Output: installer/Output/HifehTastyKitchen_Setup.exe
; ============================================================================

#define MyAppName       "Hifeh Tasty Kitchen"
#define MyAppVersion    "1.0.0"
#define MyAppPublisher  "Hifeh Tasty Kitchen"
#define MyAppURL        "http://localhost:8080"
#define MyAppExeName    "posd.exe"

[Setup]
; Unique App ID — do NOT change after first release (used for upgrades)
AppId={{A7F3B2C1-4D5E-6F78-9A0B-C1D2E3F4A5B6}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
; Allow user to choose install directory
DisableDirPage=no
; Allow user to choose start menu group
DisableProgramGroupPage=no
; License / info pages (optional — comment out if not needed)
; LicensFile=LICENSE.txt
InfoBeforeFile=..\README.md
; Output settings
OutputDir=Output
OutputBaseFilename=HifehTastyKitchen_Setup_v{#MyAppVersion}
; Compression
Compression=lzma2/ultra64
SolidCompression=yes
; Require admin rights (needed for Program Files + firewall rule)
PrivilegesRequired=admin
; Installer appearance
SetupIconFile=app.ico
WizardStyle=modern
WizardSizePercent=110
; Uninstall settings
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName}
; Minimum Windows version (Windows 10+)
MinVersion=10.0

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon";  Description: "Create a &Desktop shortcut";        GroupDescription: "Additional shortcuts:"; Flags: checked
Name: "launchapp";     Description: "Launch {#MyAppName} after install"; GroupDescription: "After installation:";    Flags: checked
Name: "firewall";      Description: "Allow LAN access (Windows Firewall rule)"; GroupDescription: "Network:"; Flags: checked

[Files]
; Main executable — ADJUST THIS PATH if your posd.exe is elsewhere
Source: "..\posd.exe";    DestDir: "{app}"; Flags: ignoreversion
; README
Source: "..\README.md";   DestDir: "{app}"; Flags: ignoreversion
; Logo (for reference)
Source: "..\web\static\images\logo.png"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Start Menu
Name: "{group}\{#MyAppName}";            Filename: "{app}\{#MyAppExeName}"; Comment: "Start the POS server"
Name: "{group}\Open Till (Browser)";     Filename: "{#MyAppURL}";           Comment: "Open the till in your browser"
Name: "{group}\Uninstall {#MyAppName}";  Filename: "{uninstallexe}"
; Desktop (if selected)
Name: "{commondesktop}\{#MyAppName}";    Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon; Comment: "Start the POS server"

[Run]
; Launch after install (if selected)
Filename: "{app}\{#MyAppExeName}"; Description: "Launch {#MyAppName}"; Flags: nowait postinstall skipifsilent; Tasks: launchapp
; Open browser after launch
Filename: "{#MyAppURL}"; Description: "Open the Till"; Flags: shellexec nowait postinstall skipifsilent runasoriginaluser; Tasks: launchapp
; Add firewall rule (if selected)
Filename: "netsh"; Parameters: "advfirewall firewall add rule name=""{#MyAppName}"" dir=in action=allow program=""{app}\{#MyAppExeName}"" enable=yes profile=private"; Flags: runhidden; Tasks: firewall

[UninstallRun]
; Remove firewall rule on uninstall
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""{#MyAppName}"""; Flags: runhidden

[UninstallDelete]
; Clean up database files on uninstall (optional — user can back up first)
; Comment these out if you want to preserve data after uninstall
; Type: files; Name: "{app}\pos.db"
; Type: files; Name: "{app}\pos.db-shm"
; Type: files; Name: "{app}\pos.db-wal"

[Messages]
WelcomeLabel2=This will install {#MyAppName} on your computer.%n%nThe POS system runs offline — no internet needed. Just install, launch, and start selling!
FinishedLabel=Setup has finished installing {#MyAppName}.%n%nThe app is now running. Open Google Chrome and go to http://localhost:8080 to access the till.%n%nDefault Owner PIN: 123456

[Code]
// Show a warning if posd.exe is already running
function InitializeSetup(): Boolean;
var
  ResultCode: Integer;
begin
  Result := True;
  // Check if posd.exe is running
  if Exec('tasklist', '/FI "IMAGENAME eq posd.exe" /NH', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
  begin
    // We can't easily parse output in Pascal, so just warn the user
    if MsgBox('{#MyAppName} might already be running.' + #13#10 + #13#10 +
              'Please close it before continuing (close the black terminal window).' + #13#10 + #13#10 +
              'Click OK to continue anyway, or Cancel to exit setup.',
              mbInformation, MB_OKCANCEL) = IDCANCEL then
    begin
      Result := False;
    end;
  end;
end;

// Kill any running instance before installing files
procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
begin
  if CurStep = ssInstall then
  begin
    Exec('taskkill', '/F /IM posd.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end;
end;
