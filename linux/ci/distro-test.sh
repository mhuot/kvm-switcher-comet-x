#!/bin/sh
# Installs the distribution's GTK/AppIndicator/libsecret packages, then runs the Linux test
# suite and the GUI self-test (under Xvfb with an unlocked throwaway GNOME Keyring).
# Runs as root inside a distro container; used by CI and by linux/ci/run-local.sh.
#   usage: distro-test.sh <path-to-repo>
set -eu

repo_path="${1:?usage: distro-test.sh <path-to-repo>}"
# shellcheck source=/dev/null
. /etc/os-release
echo "=== ${PRETTY_NAME}"

case "${ID} ${ID_LIKE:-}" in
    *debian*|*ubuntu*)
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq
        apt-get install -y -qq --no-install-recommends \
            python3 python3-venv python3-pip python3-gi gir1.2-gtk-3.0 \
            gir1.2-ayatanaappindicator3-0.1 gir1.2-secret-1 \
            xvfb xauth dbus dbus-x11 gnome-keyring adwaita-icon-theme ca-certificates >/dev/null
        ;;
    *fedora*)
        dnf install -y -q python3 python3-gobject gobject-introspection gtk3 \
            libayatana-appindicator-gtk3 \
            libsecret xorg-x11-server-Xvfb xvfb-run dbus-daemon dbus-tools gnome-keyring \
            adwaita-icon-theme >/dev/null
        ;;
    *arch*)
        # pacman's download sandbox fails in some container runtimes (seccomp/emulation).
        sed -i 's/^#DisableSandboxSyscalls/DisableSandboxSyscalls/; s/^DownloadUser/#DownloadUser/' \
            /etc/pacman.conf
        pacman -Sy --noconfirm --needed --quiet python python-gobject gtk3 \
            libayatana-appindicator libsecret xorg-server-xvfb xorg-xauth dbus gnome-keyring \
            adwaita-icon-theme which >/dev/null
        ;;
    *suse*)
        zypper --non-interactive --quiet install python3 python3-gobject python3-gobject-Gdk \
            typelib-1_0-Gtk-3_0 typelib-1_0-AyatanaAppIndicator3-0_1 typelib-1_0-Secret-1 \
            xvfb-run dbus-1-daemon gnome-keyring adwaita-icon-theme >/dev/null
        ;;
    *)
        echo "Unsupported distribution: ${ID}" >&2
        exit 2
        ;;
esac

# Work on a copy so the mounted checkout stays clean.
work_path="$(mktemp -d)"
cp -R "${repo_path}/linux" "${repo_path}/fixtures" "${work_path}/"
cd "${work_path}/linux"
# Drop anything built on the host (e.g. a macOS venv) so the container starts clean.
rm -rf .venv build src/*.egg-info
find . -name __pycache__ -type d -prune -exec rm -rf {} +

# System site packages give the venv the distribution's PyGObject (CLAUDE.md: always a venv).
python3 -m venv --system-site-packages .venv
.venv/bin/python -m pip install --quiet --upgrade pip
.venv/bin/python -m pip install --quiet '.[dev]'

echo "--- pytest"
.venv/bin/python -m pytest -q -p no:cacheprovider

echo "--- self-test (Xvfb + D-Bus session + unlocked GNOME Keyring)"
export NO_AT_BRIDGE=1  # silence the accessibility-bus warning in headless runs
dbus-run-session -- sh -c '
    printf "ci-keyring-password" | gnome-keyring-daemon --unlock --components=secrets >/dev/null
    xvfb-run -a .venv/bin/kvm-switcher self-test
'
