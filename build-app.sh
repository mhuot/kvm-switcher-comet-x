#!/bin/zsh
# Builds "KVM Switcher.app" into ./build. Pass --install to copy it to /Applications.
set -euo pipefail
cd "${0:A:h}"

app_bundle="build/KVM Switcher.app"
swift build -c release --arch arm64 --arch x86_64
binary_dir="$(swift build -c release --arch arm64 --arch x86_64 --show-bin-path)"

rm -rf "$app_bundle"
mkdir -p "$app_bundle/Contents/MacOS"
cp "$binary_dir/KVMSwitcher" "$app_bundle/Contents/MacOS/KVMSwitcher"
cp Resources/Info.plist "$app_bundle/Contents/Info.plist"
codesign --force --sign - "$app_bundle"
echo "Built $app_bundle"

if [[ "${1:-}" == "--install" ]]; then
    pkill -x KVMSwitcher 2>/dev/null || true
    rm -rf "/Applications/KVM Switcher.app"
    cp -R "$app_bundle" /Applications/
    open "/Applications/KVM Switcher.app"
    echo "Installed to /Applications"
fi
