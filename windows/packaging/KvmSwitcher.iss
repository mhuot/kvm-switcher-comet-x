; Inno Setup script for the KVM Switcher installer, one per architecture.
;   ISCC.exe /DAppVersion=1.0.0 /DArch=x64 windows\packaging\KvmSwitcher.iss
; Defines: AppVersion (required), Arch (x64 or arm64; default x64), ExeDir (folder holding
; KVMSwitcher.exe; default windows\publish\win-<Arch>), OutDir (default windows\publish).
; /DSign plus a sign tool named "signtool" (see sign.ps1) signs the installer and its
; uninstaller as they are compiled: unins000.exe only exists during the compile.

#ifndef AppVersion
  #error Pass the version: /DAppVersion=<version>
#endif
#ifndef Arch
  #define Arch "x64"
#endif
#ifndef ExeDir
  #define ExeDir AddBackslash(SourcePath) + "..\publish\win-" + Arch
#endif
#ifndef OutDir
  #define OutDir AddBackslash(SourcePath) + "..\publish"
#endif

[Setup]
; The same AppId for both architectures, so either one upgrades or uninstalls the other.
AppId={{50E12EBF-1E4E-43BB-B4EC-D457C8933EE9}
AppName=KVM Switcher
AppVersion={#AppVersion}
AppVerName=KVM Switcher {#AppVersion}
AppPublisher=Mike Huot
AppPublisherURL=https://github.com/mhuot/kvm-switcher-comet-x
AppSupportURL=https://github.com/mhuot/kvm-switcher-comet-x/issues
VersionInfoVersion={#AppVersion}
VersionInfoProductName=KVM Switcher
; Per-user install by default, so no admin rights are needed; the dialog offers all users too.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
DefaultDirName={autopf}\KVM Switcher
DisableProgramGroupPage=yes
UninstallDisplayName=KVM Switcher
UninstallDisplayIcon={app}\KVMSwitcher.exe
OutputDir={#OutDir}
OutputBaseFilename=KVMSwitcher-{#AppVersion}-win-{#Arch}-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
#if Arch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif
#ifdef Sign
SignTool=signtool
SignedUninstaller=yes
SignToolRetryCount=3
#endif

[Files]
Source: "{#ExeDir}\KVMSwitcher.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\KVM Switcher"; Filename: "{app}\KVMSwitcher.exe"

[Registry]
; The app writes this "Launch at login" value itself; writing nothing here just lets
; uninstall remove it.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueName: "KVMSwitcher"; ValueType: none; Flags: uninsdeletevalue dontcreatekey

[Run]
Filename: "{app}\KVMSwitcher.exe"; Description: "Start KVM Switcher"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM KVMSwitcher.exe"; Flags: runhidden; RunOnceId: "StopKVMSwitcher"
