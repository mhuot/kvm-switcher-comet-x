# KVM Switcher for Linux

Panel app and command line tool for switching ports on a GL.iNet Comet X (GL-RM4PE) KVM.
It's a single static Go binary with no runtime dependencies: it puts its icon in the panel
through the StatusNotifierItem D-Bus protocol and keeps the password in the desktop keyring
through the Secret Service D-Bus API. There's no GTK, no libappindicator and no OpenGL.

Users: see the [Linux user guide](../docs/USER_GUIDE_LINUX.md).

| Task | Command (from `linux/`) |
|---|---|
| Run the tests | `go test -race ./...` |
| Lint | `gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...` |
| Build binaries, tarballs, `.deb`, `.rpm` and Arch packages | `./build.sh` (output in `dist/`) |
| Test the packages on every distribution in Docker | `./ci/run-local.sh` |
| Check the panel icon and keyring on this machine | `kvm-switcher self-test` |

| Package | What it does |
|---|---|
| `cmd/kvm-switcher` | Command line: `tray`, `switch`, `status`, `list`, `setup`, `self-test` |
| `internal/kvm` | KVM API client and certificate pinning (see [docs/API.md](../docs/API.md)) |
| `internal/settings` | `~/.config/kvm-switcher/settings.json` |
| `internal/credentials` | Keyring (Secret Service), environment, systemd credential and password-file sources |
| `internal/statusnotifier` | Panel icon, label and menu (StatusNotifierItem + `com.canonical.dbusmenu`) |
| `internal/tray` | The panel app: refresh, switching, menu layout, icons |
| `internal/setup` | Settings as a terminal wizard or zenity/kdialog dialogs |
| `internal/instance` | One tray per session; `switch N` and `setup` reach it over D-Bus |
| `internal/selftest` | `kvm-switcher self-test`, which CI runs in every distribution |
