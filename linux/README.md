# KVM Switcher for Linux

Panel (tray) app and command line tool for switching ports on a GL.iNet KVM. Python + GTK 3 +
AppIndicator, with the password stored in the desktop keyring through libsecret.

- Install: `./install.sh` (see the [Linux user guide](../docs/USER_GUIDE_LINUX.md))
- Keyboard shortcuts: bind `kvm-switcher switch N` in your desktop's keyboard settings
- Tests: `python3 -m venv --system-site-packages .venv && .venv/bin/pip install -e '.[dev]' && .venv/bin/pytest`
- Lint: `.venv/bin/black --check src tests && .venv/bin/pylint src tests`
- All distributions in Docker: `./ci/run-local.sh`
