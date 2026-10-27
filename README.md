# KVM Switcher

Switch the active port on a [GL.iNet Comet X (GL-RM4PE)](https://www.gl-inet.com/blogs/blog/gl-inet-launches-comet-x-gl-rm4pe-a-quad-port-remote-kvm-with-poe-for-simplified-server-management), the 4-port remote KVM,
from the macOS menu bar, the Windows 11 system tray, or a Linux desktop panel.

KVM Switcher is an independent project and is not affiliated with GL.iNet.

**Download:** get the Mac .dmg, the Windows installer or exe, or the Linux package from
[Releases](https://github.com/mhuot/kvm-switcher-comet-x/releases/latest), then follow the user guide for your platform.

Each platform is a separate native app with no shared code. They all implement the same
[API contract](docs/API.md) and are tested against the same [fixtures](fixtures/).

| Platform | Folder | Built with | User guide |
|---|---|---|---|
| macOS 13+ | [`macos/`](macos) | Swift, AppKit + SwiftUI | [USER_GUIDE_MACOS.md](docs/USER_GUIDE_MACOS.md) |
| Windows 11 (x64, Arm64) | [`windows/`](windows) | C#, .NET 10 WinForms | [USER_GUIDE_WINDOWS.md](docs/USER_GUIDE_WINDOWS.md) |
| Linux (x86_64, arm64) | [`linux/`](linux) | Go, StatusNotifierItem over D-Bus (no GTK) | [USER_GUIDE_LINUX.md](docs/USER_GUIDE_LINUX.md) |

## Features

| | macOS | Windows 11 | Linux |
|---|---|---|---|
| Where it lives | Menu bar, active port name beside the icon | Tray, port number drawn on the icon, name on hover | Panel, active port name beside the icon |
| Switch from the menu | Click, or press 1–4 | Click, or press 1–4 | Click, or press 1–4 |
| Global shortcut | ⌃⌥⌘1–4 | Ctrl+Alt+Win+1–4 | Desktop shortcut running `kvm-switcher switch N` |
| Password storage | Keychain | Credential Manager | Desktop keyring (Secret Service); file or environment when headless |
| Start at login | Yes | Yes | Yes (XDG autostart) |
| Command line | | | `kvm-switcher switch/status/list/setup`, also headless |

All three read port names from the KVM's web UI and pin the KVM's self-signed certificate on
first use. None of them support KVMs with 2FA turned on.

Release builds are signed: the Mac app and `.dmg` with a Developer ID and notarized by Apple,
the Windows exes and installers with Azure Artifact Signing. See [SIGNING.md](docs/SIGNING.md).

## Repository layout

```
macos/      Swift package: app, tests, build-app.sh, make-dmg.sh
windows/    KvmSwitcher (app), KvmSwitcher.Tests (portable tests), build.sh, packaging/ (installer, signing)
linux/      Go app (cmd/, internal/), build.sh, install.sh, packaging/, ci/ (distro tests)
fixtures/   Anonymized KVM responses + cases.json: the expected results every test suite checks
docs/       API contract, code signing and the three user guides
.github/    CI workflow and Dependabot
```

## Build and test

| Platform | Build | Test |
|---|---|---|
| macOS | `./macos/build-app.sh` (`--install` copies it to /Applications) | `cd macos && swift test` |
| Windows | `./windows/build.sh` (Docker, from macOS/Linux) | `dotnet test windows/KvmSwitcher.Tests` (any OS) |
| Linux | `./linux/build.sh` (static binaries, tarballs, `.deb`, `.rpm`, Arch) | `cd linux && go test ./...`; all distros: `./linux/ci/run-local.sh` |

`macos/build-app.sh` signs with your Apple Development or Developer ID identity when one is
installed (so the Keychain's "Always Allow" survives rebuilds), otherwise ad-hoc.
`windows/build.sh` writes self-contained single-file builds to
`windows/publish/win-x64/KVMSwitcher.exe` and `windows/publish/win-arm64/KVMSwitcher.exe`.
`KVMSwitcher.exe --self-test` checks the Windows-only parts without a KVM.
`./macos/make-dmg.sh` packages the built app as a `.dmg`, and on Windows
`windows/packaging/make-installers.ps1` builds the Inno Setup installers from the exes.
[SIGNING.md](docs/SIGNING.md) covers signing either one locally.

## Continuous integration

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs on every push and pull request:

| Job | What it checks |
|---|---|
| Lint | shellcheck, gofmt, go vet, staticcheck, fixture JSON |
| macOS | `swift test`, universal app bundle (artifact), unsigned `.dmg` |
| macOS signed | On `v*` tags and manual runs: signs, notarizes and staples the app and `.dmg`, then checks them (artifact) |
| Windows client tests | the portable xUnit suite on Ubuntu, macOS and Windows |
| Windows | x64 build + `--self-test` on Windows; Arm64 too on public repositories (artifacts) |
| Windows installers | Builds both installers, then installs and uninstalls the x64 one quietly |
| Windows signed | On `v*` tags and manual runs: signs both exes, builds both installers signed (uninstallers too), then checks every signature (artifact) |
| Linux tests | `go test -race` (API client against a fake KVM, settings, setup wizard, CLI) |
| Linux package | static amd64/arm64 binaries, tarballs, `.deb`, `.rpm` and Arch packages (artifact) |
| Linux distros | Installs the package on Ubuntu 22.04/24.04, Debian 12/13, Fedora, Arch and openSUSE Tumbleweed (plus three on Arm64), then runs the CLI, the tarball installer and a D-Bus self-test of the panel icon, menu and keyring |
| GitHub Release | On `v*` tags (or workflow dispatch with release enabled), after every job above passes: checks the app versions match the tag, then publishes the Mac `.dmg`, both Windows installers and exes, the Linux packages and tarballs, and `SHA256SUMS.txt` |

### Making a release

1. Set the same version in `macos/Resources/Info.plist` (`CFBundleShortVersionString`),
   `windows/KvmSwitcher/KvmSwitcher.csproj` (`<Version>`) and `linux/cmd/kvm-switcher/main.go`
   (`var version`), and commit.
2. Tag and push: `git tag v1.0.0 && git push origin v1.0.0` (or trigger the CI workflow with `release: true`).

The Mac and Windows builds are signed only when the repository has the signing secrets and the
`release` environment described in [SIGNING.md](docs/SIGNING.md); without them they're published unsigned.

## Privacy

Do not commit `.har` captures: they contain live session tokens (`*.har` is gitignored).
The fixtures are anonymized.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
