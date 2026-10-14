# KVM Switcher

Switch the active port on a GL.iNet KVM (GLKVM / RM4PE-style units with the built-in 4-port
switch) from the macOS menu bar or the Windows 11 system tray.

| Platform | Source | User guide |
|---|---|---|
| macOS 13+ | `Sources/KVMSwitcher` (Swift, AppKit + SwiftUI) | [docs/USER_GUIDE.md](docs/USER_GUIDE.md) |
| Windows 11 | `windows/KvmSwitcher` (C#, .NET 10 WinForms) | [docs/USER_GUIDE_WINDOWS.md](docs/USER_GUIDE_WINDOWS.md) |

## Features (macOS)

- Menu bar shows the active port's name (or number, or just the icon); the menu lists ports with a checkmark on the active one
- Press 1–4 while the menu is open, or use the global hotkeys ⌃⌥⌘1 – ⌃⌥⌘4 from anywhere
- Port names come from the KVM, or from local names you set in Settings
- Admin password stored in the macOS Keychain
- Pins the KVM's self-signed TLS certificate on first connect (trust on first use)
- Optional launch at login

See the [User Guide](docs/USER_GUIDE.md) for setup, everyday use, troubleshooting and the
macOS Keychain prompt.

## Features (Windows 11)

- Tray icon with the active port number drawn on it; hover for the port name
- Left- or right-click menu with a checkmark on the active port; press 1–4 while it is open
- Global hotkeys Ctrl+Alt+Win+1 – 4, with an optional "Switched to …" notification
- Password stored in Windows Credential Manager; same certificate pinning as macOS
- Optional launch at login (per-user Run key), single `.exe`, no installer or admin rights

## Build & install (macOS)

```sh
./build-app.sh            # builds build/KVM Switcher.app (universal; signed with your
                          # Apple Development/Developer ID identity if present, else ad-hoc)
./build-app.sh --install  # also copies it to /Applications and launches it
```

Requires Xcode / Swift 5.10+ and macOS 13+. On first launch, enter the KVM address (e.g.
`glkvm.local` or its IP address) and the admin password in Settings, then click **Test Connection**.

## Build (Windows)

```sh
./windows/build.sh   # needs Docker; builds in the .NET 10 SDK container
```

Produces self-contained single-file builds at `windows/publish/win-x64/KVMSwitcher.exe` and
`windows/publish/win-arm64/KVMSwitcher.exe` (no .NET runtime needed on the target PC). On a
Windows machine with the .NET 10 SDK you can instead run, from `windows/KvmSwitcher`:

```powershell
dotnet publish -c Release -r win-x64 --self-contained -p:PublishSingleFile=true
```

## KVM API used

The GLKVM firmware is derived from PiKVM's kvmd. All replies are `{"ok": bool, "result": …}`.

| Purpose      | Request                                                            |
|--------------|--------------------------------------------------------------------|
| Log in       | `POST /api/auth/login`, form body `user=admin&passwd=…` → `{token}` |
| Switch state | `GET /api/switch`, header `token: …`                               |
| Change port  | `POST /api/switch/set_active?port=N` (N is zero-based)             |
| Port names   | `GET /api/system/get_config` → `result.config.port_names[]`        |

`result.summary.active_port` is the active zero-based port, and `result.model.ports[]` lists the ports.
The port names you set in the web UI live in the UI's config (`port_names`), not in `model.ports[].name`.
Accounts with 2FA enabled are not supported.

Do not commit `.har` captures: they contain live session tokens (they are gitignored).
