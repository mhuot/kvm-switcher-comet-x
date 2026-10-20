#!/bin/sh
# Installs the built KVM Switcher package for this distribution, then checks the command,
# the menu entry, the per-user tarball installer and the D-Bus self-test (panel icon, menu
# and an unlocked throwaway GNOME Keyring). No GTK or Python is installed: only D-Bus and
# a keyring daemon for the test itself.
# Runs as root inside a distro container; used by CI and by linux/ci/run-local.sh.
#   usage: distro-test.sh <path-to-linux/dist>
set -eu

dist_path="${1:?usage: distro-test.sh <path-to-linux/dist>}"
# shellcheck source=/dev/null
. /etc/os-release
echo "=== ${PRETTY_NAME}"

case "$(uname -m)" in
    x86_64) deb_arch=amd64 rpm_arch=x86_64 ;;
    aarch64) deb_arch=arm64 rpm_arch=aarch64 ;;
    *) echo "Unsupported machine $(uname -m)" >&2; exit 2 ;;
esac

case "${ID} ${ID_LIKE:-}" in
    *debian*|*ubuntu*)
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq
        apt-get install -y -qq --no-install-recommends dbus dbus-x11 gnome-keyring procps \
            "${dist_path}"/kvm-switcher_*_"${deb_arch}".deb >/dev/null
        ;;
    *fedora*)
        dnf install -y -q dbus-daemon dbus-tools gnome-keyring procps-ng \
            "${dist_path}"/kvm-switcher-*."${rpm_arch}".rpm >/dev/null
        ;;
    *arch*)
        # pacman's download sandbox fails in some container runtimes (seccomp/emulation).
        sed -i 's/^#DisableSandboxSyscalls/DisableSandboxSyscalls/; s/^DownloadUser/#DownloadUser/' \
            /etc/pacman.conf
        pacman -Sy --noconfirm --needed --quiet dbus gnome-keyring procps-ng >/dev/null
        pacman -U --noconfirm "${dist_path}"/kvm-switcher-*-"${rpm_arch}".pkg.tar.zst >/dev/null
        ;;
    *suse*)
        zypper --non-interactive --quiet install dbus-1-daemon dbus-1-tools gnome-keyring procps tar gzip >/dev/null
        zypper --non-interactive --quiet --no-gpg-checks install --allow-unsigned-rpm \
            "${dist_path}"/kvm-switcher-*."${rpm_arch}".rpm >/dev/null
        ;;
    *)
        echo "Unsupported distribution: ${ID}" >&2
        exit 2
        ;;
esac

failures=0
expect() {
    description="$1"
    shift
    if "$@"; then
        echo "PASS $description"
    else
        echo "FAIL $description"
        failures=$((failures + 1))
    fi
}

expect "package installs /usr/bin/kvm-switcher" test -x /usr/bin/kvm-switcher
expect "package installs the menu entry" test -f /usr/share/applications/net.mhuot.KVMSwitcher.desktop
echo "version $(kvm-switcher --version)"
empty_home="$(mktemp -d)"
expect "unconfigured status explains what to set" \
    sh -c "HOME='$empty_home' kvm-switcher status 2>&1 | grep -q 'Set the KVM address'"

echo "--- per-user tarball install"
tarball_home="$(mktemp -d)"
tar -xzf "${dist_path}"/kvm-switcher-*-linux-"${deb_arch}".tar.gz -C "$tarball_home"
expect "install.sh installs into ~/.local" sh -c "HOME='$tarball_home' '$tarball_home'/kvm-switcher-*/install.sh >/dev/null \
    && test -x '$tarball_home/.local/bin/kvm-switcher' \
    && grep -q '^Exec=$tarball_home/.local/bin/kvm-switcher' '$tarball_home/.local/share/applications/net.mhuot.KVMSwitcher.desktop'"
expect "install.sh --uninstall removes it" sh -c "HOME='$tarball_home' '$tarball_home'/kvm-switcher-*/install.sh --uninstall >/dev/null \
    && test ! -e '$tarball_home/.local/bin/kvm-switcher'"

echo "--- self-test (D-Bus session + unlocked GNOME Keyring)"
self_test_log="$(mktemp)"
# gnome-keyring-daemon needs ~/.cache for its socket and ~/.local/share/keyrings to create the
# login keyring; some images have neither.
mkdir -p "${HOME:-/root}/.cache" "${HOME:-/root}/.local/share/keyrings"
# Wait for the unlocked daemon to own its bus name; otherwise D-Bus activation can start a
# second, locked daemon that wants a password prompt.
dbus-run-session -- sh -c '
    printf "ci-keyring-password" | gnome-keyring-daemon --unlock --components=secrets >/dev/null
    for attempt in 1 2 3 4 5 6 7 8 9 10; do
        dbus-send --session --print-reply --dest=org.freedesktop.DBus /org/freedesktop/DBus \
            org.freedesktop.DBus.NameHasOwner string:org.freedesktop.secrets 2>/dev/null \
            | grep -q "boolean true" && break
        sleep 0.5
    done
    kvm-switcher self-test
' >"$self_test_log" 2>&1 || true
cat "$self_test_log"
expect "self-test passes" grep -q "^RESULT PASS" "$self_test_log"
expect "panel icon works over D-Bus" grep -q "^PASS panel icon over D-Bus" "$self_test_log"
expect "keyring round trip works" grep -q "^PASS keyring round trip" "$self_test_log"

if [ "$failures" -gt 0 ]; then
    echo "${failures} check(s) failed on ${PRETTY_NAME}"
    exit 1
fi
echo "All checks passed on ${PRETTY_NAME}"
