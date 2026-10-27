#!/usr/bin/env bash
# Packages "KVM Switcher.app" (from build-app.sh) as a drag-to-Applications .dmg.
#   ./macos/make-dmg.sh [APP_BUNDLE] [DMG_PATH]
#   defaults: build/KVM Switcher.app and build/KVMSwitcher-<version>.dmg
# Signing and notarization are optional and run only when these are set (CI secrets):
#   APPLE_SIGN_IDENTITY   e.g. "Developer ID Application: <Name> (<TEAMID>)"
#   APPLE_ID, APPLE_TEAM_ID, APPLE_APP_PASSWORD   for notarytool (with APPLE_SIGN_IDENTITY)
set -euo pipefail
script_dir="$(cd "$(dirname "$0")" && pwd)"

app_version="$(/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$script_dir/Resources/Info.plist")"
app_bundle="${1:-$script_dir/build/KVM Switcher.app}"
dmg_path="${2:-$script_dir/build/KVMSwitcher-$app_version.dmg}"

[[ -d "$app_bundle" ]] || { echo "Not found: $app_bundle (run build-app.sh first)" >&2; exit 1; }
mkdir -p "$(dirname "$dmg_path")"

signing_identity="${APPLE_SIGN_IDENTITY:-}"
should_notarize=""
if [[ -n "$signing_identity" && -n "${APPLE_ID:-}" && -n "${APPLE_TEAM_ID:-}" && -n "${APPLE_APP_PASSWORD:-}" ]]; then
    should_notarize=1
fi

notarize() {
    xcrun notarytool submit "$1" --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" \
        --password "$APPLE_APP_PASSWORD" --wait
}

# The app is a single Swift binary with no plug-ins or embedded interpreter, so the hardened
# runtime needs no entitlements.
if [[ -n "$signing_identity" ]]; then
    echo "Signing the app with: $signing_identity"
    codesign --force --deep --options runtime --timestamp --sign "$signing_identity" "$app_bundle"
    codesign --verify --deep --strict "$app_bundle"
else
    echo "APPLE_SIGN_IDENTITY not set; packaging the app as it is (not notarized)."
fi

staging_dir="$(mktemp -d)"
trap 'rm -rf "$staging_dir"' EXIT

# The app gets its own ticket as well as the .dmg, so once it's in Applications it opens the
# first time even on a Mac with no internet, where Gatekeeper can't look the ticket up.
if [[ -n "$should_notarize" ]]; then
    echo "Notarizing the app"
    ditto -c -k --keepParent "$app_bundle" "$staging_dir/app.zip"
    notarize "$staging_dir/app.zip"
    rm "$staging_dir/app.zip"
    xcrun stapler staple "$app_bundle"
fi

ditto "$app_bundle" "$staging_dir/KVM Switcher.app"
ln -s /Applications "$staging_dir/Applications"
rm -f "$dmg_path"
hdiutil create -volname "KVM Switcher" -srcfolder "$staging_dir" -ov -format UDZO "$dmg_path" >/dev/null

if [[ -n "$signing_identity" ]]; then
    codesign --force --timestamp --sign "$signing_identity" "$dmg_path"
fi
if [[ -n "$should_notarize" ]]; then
    echo "Notarizing the .dmg"
    notarize "$dmg_path"
    xcrun stapler staple "$dmg_path"
    # The copy inside the .dmg carries its own ticket.
    xcrun stapler validate "$staging_dir/KVM Switcher.app"
fi
echo "Built $dmg_path"
