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
# A stable identity keeps the Keychain's "Always Allow" valid across rebuilds; ad-hoc
# signatures change every build, so macOS asks for the login password again.
signing_identity="${SIGNING_IDENTITY:-}"
if [[ -z "$signing_identity" ]]; then
    signing_identity="$(security find-identity -v -p codesigning \
        | awk -F'"' '/"(Developer ID Application|Apple Development):/ { print $2; exit }')"
fi
if [[ -n "$signing_identity" ]]; then
    echo "Signing with: $signing_identity"
    codesign --force --options runtime --sign "$signing_identity" "$app_bundle"
else
    echo "No signing identity found; using ad-hoc signing (Keychain will re-prompt after each rebuild)."
    codesign --force --sign - "$app_bundle"
fi
echo "Built $app_bundle"

if [[ "${1:-}" == "--install" ]]; then
    pkill -x KVMSwitcher 2>/dev/null || true
    rm -rf "/Applications/KVM Switcher.app"
    cp -R "$app_bundle" /Applications/
    open "/Applications/KVM Switcher.app"
    echo "Installed to /Applications"
fi
