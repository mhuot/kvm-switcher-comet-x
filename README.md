# KVM Switcher

A macOS menu bar app for switching the active port on a GL.iNet KVM (GLKVM / RM4PE-style
units with the built-in 4-port switch).

## Features

- Menu bar shows the active port's name (or number, or just the icon); the menu lists ports with a checkmark on the active one
- Press 1–4 while the menu is open, or use the global hotkeys ⌃⌥⌘1 – ⌃⌥⌘4 from anywhere
- Port names come from the KVM, or from local names you set in Settings
- Admin password stored in the macOS Keychain
- Pins the KVM's self-signed TLS certificate on first connect (trust on first use)
- Optional launch at login

See the [User Guide](docs/USER_GUIDE.md) for setup, everyday use, troubleshooting and the
macOS Keychain prompt.

## Build & install

```sh
./build-app.sh            # builds build/KVM Switcher.app (universal, ad-hoc signed)
./build-app.sh --install  # also copies it to /Applications and launches it
```

Requires Xcode / Swift 5.10+ and macOS 13+. On first launch, enter the KVM address (e.g.
`glkvm.local` or its IP address) and the admin password in Settings, then click **Test Connection**.

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
