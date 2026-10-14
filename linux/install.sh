#!/bin/sh
# Installs KVM Switcher for the current user (no root needed for this step):
#   venv:     ${XDG_DATA_HOME:-~/.local/share}/kvm-switcher/venv
#   command:  ~/.local/bin/kvm-switcher
#   menu:     ~/.local/share/applications/net.mhuot.KVMSwitcher.desktop
# Usage: ./install.sh            install or update
#        ./install.sh --uninstall
set -eu

source_directory="$(cd "$(dirname "$0")" && pwd)"
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
venv_directory="$data_home/kvm-switcher/venv"
launcher_path="$HOME/.local/bin/kvm-switcher"
desktop_entry_path="$data_home/applications/net.mhuot.KVMSwitcher.desktop"

if [ "${1:-}" = "--uninstall" ]; then
    "$launcher_path" --version >/dev/null 2>&1 && pkill -f "kvm-switcher/venv/bin/kvm-switcher" || true
    rm -rf "$data_home/kvm-switcher" "$launcher_path" "$desktop_entry_path"
    rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/autostart/net.mhuot.KVMSwitcher.desktop"
    echo "KVM Switcher removed. Settings remain in ${XDG_CONFIG_HOME:-$HOME/.config}/kvm-switcher;"
    echo "the password remains in your keyring under 'KVM Switcher'."
    exit 0
fi

print_package_hint() {
    # shellcheck source=/dev/null
    . /etc/os-release 2>/dev/null || true
    echo "Install the system packages first, then re-run this script:" >&2
    case "${ID:-} ${ID_LIKE:-}" in
        *debian*|*ubuntu*)
            echo "  sudo apt install python3-venv python3-gi gir1.2-gtk-3.0 gir1.2-ayatanaappindicator3-0.1 gir1.2-secret-1" >&2 ;;
        *fedora*)
            echo "  sudo dnf install python3-gobject gtk3 libayatana-appindicator-gtk3 libsecret" >&2 ;;
        *arch*)
            echo "  sudo pacman -S python-gobject gtk3 libayatana-appindicator libsecret" >&2 ;;
        *suse*)
            echo "  sudo zypper install python3-gobject python3-gobject-Gdk typelib-1_0-Gtk-3_0 typelib-1_0-AyatanaAppIndicator3-0_1 typelib-1_0-Secret-1" >&2 ;;
        *)
            echo "  PyGObject, GTK 3, Ayatana AppIndicator (GObject introspection) and libsecret" >&2 ;;
    esac
}

if ! python3 -c 'import gi; gi.require_version("Gtk", "3.0"); from gi.repository import Gtk' 2>/dev/null; then
    echo "PyGObject with GTK 3 was not found." >&2
    print_package_hint
    exit 1
fi
if ! python3 -c '
import gi
for name in ("AyatanaAppIndicator3", "AppIndicator3"):
    try:
        gi.require_version(name, "0.1")
        break
    except ValueError:
        pass
else:
    raise SystemExit(1)' 2>/dev/null; then
    echo "AppIndicator (GObject introspection) was not found." >&2
    print_package_hint
    exit 1
fi

echo "Creating virtual environment in $venv_directory"
python3 -m venv --system-site-packages "$venv_directory"
"$venv_directory/bin/python" -m pip install --quiet --upgrade pip
"$venv_directory/bin/python" -m pip install --quiet "$source_directory"

mkdir -p "$(dirname "$launcher_path")" "$(dirname "$desktop_entry_path")"
ln -sf "$venv_directory/bin/kvm-switcher" "$launcher_path"
sed "s#@EXEC@#$launcher_path#" "$source_directory/data/net.mhuot.KVMSwitcher.desktop.in" \
    > "$desktop_entry_path"

echo "Installed. Start it from your applications menu (\"KVM Switcher\") or run: kvm-switcher"
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) echo "Note: add ~/.local/bin to your PATH to use the kvm-switcher command." ;;
esac
case "${XDG_CURRENT_DESKTOP:-}" in
    *GNOME*)
        echo "GNOME: the panel icon needs the 'AppIndicator and KStatusNotifierItem Support' extension"
        echo "(preinstalled on Ubuntu; on Fedora: sudo dnf install gnome-shell-extension-appindicator)."
        ;;
esac
