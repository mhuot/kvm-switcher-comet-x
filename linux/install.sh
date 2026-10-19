#!/bin/sh
# Installs KVM Switcher for the current user from the release tarball (no root needed):
#   command:  ~/.local/bin/kvm-switcher
#   menu:     ~/.local/share/applications/net.mhuot.KVMSwitcher.desktop
# Usage: ./install.sh            install or update
#        ./install.sh --uninstall
# Prefer the .deb, .rpm or Arch package when your distribution takes one.
set -eu

source_directory="$(cd "$(dirname "$0")" && pwd)"
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
config_home="${XDG_CONFIG_HOME:-$HOME/.config}"
binary_path="$HOME/.local/bin/kvm-switcher"
desktop_entry_path="$data_home/applications/net.mhuot.KVMSwitcher.desktop"
# Left by the earlier Python version of KVM Switcher.
legacy_python_directory="$data_home/kvm-switcher"

stop_running_tray() {
    pkill -x kvm-switcher 2>/dev/null || true
    pkill -f "kvm-switcher/venv/bin/kvm-switcher" 2>/dev/null || true
}

if [ "${1:-}" = "--uninstall" ]; then
    stop_running_tray
    rm -rf "$legacy_python_directory"
    rm -f "$binary_path" "$desktop_entry_path" "$config_home/autostart/net.mhuot.KVMSwitcher.desktop"
    echo "KVM Switcher removed. Settings remain in $config_home/kvm-switcher;"
    echo "the password remains in your keyring under 'KVM Switcher'."
    exit 0
fi

if [ ! -x "$source_directory/kvm-switcher" ]; then
    echo "No kvm-switcher binary next to this script. Download a release tarball from" >&2
    echo "https://github.com/mhuot/kvm-switcher-comet-x/releases, or build one with linux/build.sh." >&2
    exit 1
fi

was_running=false
if pgrep -x kvm-switcher >/dev/null 2>&1 || pgrep -f "kvm-switcher/venv/bin/kvm-switcher" >/dev/null 2>&1; then
    was_running=true
    stop_running_tray
fi
if [ -d "$legacy_python_directory/venv" ]; then
    echo "Removing the earlier Python version from $legacy_python_directory"
    rm -rf "$legacy_python_directory"
fi

mkdir -p "$(dirname "$binary_path")" "$(dirname "$desktop_entry_path")"
# Remove first: the Python version left a symlink here, and copying onto it would write
# through to its target.
rm -f "$binary_path"
cp "$source_directory/kvm-switcher" "$binary_path"
chmod 755 "$binary_path"
sed "s#^Exec=.*#Exec=$binary_path#" "$source_directory/net.mhuot.KVMSwitcher.desktop" > "$desktop_entry_path"

echo "Installed $("$binary_path" --version). Start it from your applications menu (\"KVM Switcher\") or run: kvm-switcher"
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *) echo "Note: add ~/.local/bin to your PATH to use the kvm-switcher command." ;;
esac
case "${XDG_CURRENT_DESKTOP:-}" in
    *GNOME*)
        echo "GNOME: the panel icon needs the 'AppIndicator and KStatusNotifierItem Support' extension"
        echo "(already on in Ubuntu)."
        ;;
esac
if [ "$was_running" = true ] && { [ -n "${WAYLAND_DISPLAY:-}" ] || [ -n "${DISPLAY:-}" ]; }; then
    nohup "$binary_path" >/dev/null 2>&1 &
    echo "Restarted the panel icon."
fi
