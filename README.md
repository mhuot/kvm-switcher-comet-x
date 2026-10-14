# KVM Switcher

Switch the active port on a GL.iNet KVM (GLKVM / RM-series units with the built-in 4-port
switch) from the macOS menu bar, the Windows 11 system tray, or a Linux desktop panel.

Each platform is a separate native app with no shared code. They all implement the same
[API contract](docs/API.md) and are tested against the same [fixtures](fixtures/).

| Platform | Folder | Built with | User guide |
|---|---|---|---|
| macOS 13+ | [`macos/`](macos) | Swift, AppKit + SwiftUI | [USER_GUIDE_MACOS.md](docs/USER_GUIDE_MACOS.md) |
| Windows 11 (x64, Arm64) | [`windows/`](windows) | C#, .NET 10 WinForms | [USER_GUIDE_WINDOWS.md](docs/USER_GUIDE_WINDOWS.md) |
| Linux | [`linux/`](linux) | Python, GTK 3 + AppIndicator | [USER_GUIDE_LINUX.md](docs/USER_GUIDE_LINUX.md) |

## Features

| | macOS | Windows 11 | Linux |
|---|---|---|---|
| Where it lives | Menu bar, active port name beside the icon | Tray, port number drawn on the icon, name on hover | Panel, active port name beside the icon |
| Switch from the menu | Click, or press 1–4 | Click, or press 1–4 | Click, or press 1–4 |
| Global shortcut | ⌃⌥⌘1–4 | Ctrl+Alt+Win+1–4 | Desktop shortcut running `kvm-switcher switch N` |
| Password storage | Keychain | Credential Manager | Desktop keyring (libsecret) |
| Start at login | Yes | Yes | Yes (XDG autostart) |
| Command line | | | `kvm-switcher switch/status/list` |

All three read port names from the KVM's web UI and pin the KVM's self-signed certificate on
first use. None of them support KVMs with 2FA turned on.

## Repository layout

```
macos/      Swift package: app, tests, build-app.sh
windows/    KvmSwitcher (app), KvmSwitcher.Tests (portable tests), build.sh
linux/      Python package, tests, install.sh, ci/ (distro test scripts)
fixtures/   Anonymized KVM responses + cases.json: the expected results every test suite checks
docs/       API contract and the three user guides
.github/    CI workflow and Dependabot
```

## Build and test

| Platform | Build | Test |
|---|---|---|
| macOS | `./macos/build-app.sh` (`--install` copies it to /Applications) | `cd macos && swift test` |
| Windows | `./windows/build.sh` (Docker, from macOS/Linux) | `dotnet test windows/KvmSwitcher.Tests` (any OS) |
| Linux | `./linux/install.sh` | see [linux/README.md](linux/README.md); all distros: `./linux/ci/run-local.sh` |

`macos/build-app.sh` signs with your Apple Development or Developer ID identity when one is
installed (so the Keychain's "Always Allow" survives rebuilds), otherwise ad-hoc.
`windows/build.sh` writes self-contained single-file builds to
`windows/publish/win-x64/KVMSwitcher.exe` and `windows/publish/win-arm64/KVMSwitcher.exe`.
`KVMSwitcher.exe --self-test` checks the Windows-only parts without a KVM.

## Continuous integration

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs on every push and pull request:

| Job | What it checks |
|---|---|
| Lint | shellcheck, black, pylint, fixture JSON |
| macOS | `swift test`, universal app bundle (artifact) |
| Windows client tests | the portable xUnit suite on Ubuntu, macOS and Windows |
| Windows | x64 build + `--self-test` on Windows; Arm64 too on public repositories (artifacts) |
| Linux distros | Ubuntu 22.04/24.04, Debian 12/13, Fedora, Arch, openSUSE Tumbleweed: pytest and a GTK/AppIndicator/keyring self-test under Xvfb |
| Linux Python versions | pytest on Python 3.10 – 3.14 |
| Linux package | sdist and wheel (artifact) |

## Privacy

Do not commit `.har` captures: they contain live session tokens (`*.har` is gitignored).
The fixtures are anonymized.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
