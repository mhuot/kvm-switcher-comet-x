# Code signing

Release builds of the Mac and Windows apps are signed, so they open without the "unidentified
developer" and "unrecognized app" warnings. The Linux packages are not signed.

| Platform | What is signed | How |
|---|---|---|
| macOS | `KVM Switcher.app` and the `.dmg` around it | Developer ID Application certificate, hardened runtime, secure timestamp; both notarized by Apple and stapled |
| Windows | `KVMSwitcher.exe` (x64, Arm64), both installers and their uninstallers | Azure Artifact Signing, Public Trust profile, signed in through GitHub's OIDC token |

The app inside the `.dmg` is notarized and stapled on its own before the `.dmg` is built, so once
it's in Applications it opens the first time even on a Mac with no internet connection.

Inno Setup signs each installer and its uninstaller (`unins000.exe`) while it compiles them,
because the uninstaller only exists during that compile.

Signing is optional. Each platform signs only when its settings exist, so forks, pull requests
and local builds still build, unsigned.

## Where it happens

| File | Role |
|---|---|
| [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) | `macos-signed` and `windows-signed` jobs: run on `v*` tags and manual runs, in the `release` environment |
| [`macos/make-dmg.sh`](../macos/make-dmg.sh) | signs, notarizes and staples the app, then builds the `.dmg` and does the same to it |
| [`windows/packaging/sign-setup.ps1`](../windows/packaging/sign-setup.ps1) | fetches signtool and the Artifact Signing client from NuGet into `windows/packaging/signing/` (ignored by git) and writes `metadata.json` |
| [`windows/packaging/sign.ps1`](../windows/packaging/sign.ps1) | signs one file; Inno Setup calls it |
| [`windows/packaging/make-installers.ps1`](../windows/packaging/make-installers.ps1) | builds one installer per architecture; `-Sign` turns on signing |
| [`windows/packaging/KvmSwitcher.iss`](../windows/packaging/KvmSwitcher.iss) | the Inno Setup script; `/DSign` sets `SignTool` and `SignedUninstaller=yes` |

Every push and pull request still builds an unsigned `.dmg` and installs and uninstalls the
unsigned x64 installer, so the packaging is tested without the signing settings.

## Settings

| Name | Kind | Value |
|---|---|---|
| `MAC_CERT_P12` | repository secret | Developer ID Application `.p12`, base64 (`base64 -i cert.p12 \| pbcopy`) |
| `MAC_CERT_PASSWORD` | repository secret | the `.p12`'s export password |
| `APPLE_SIGN_IDENTITY` | repository secret | `Developer ID Application: <Name> (<TEAMID>)` |
| `APPLE_ID` | repository secret | the Apple ID email |
| `APPLE_TEAM_ID` | repository secret | the 10-character team id |
| `APPLE_APP_PASSWORD` | repository secret | an app-specific password from appleid.apple.com, for notarytool |
| `AZURE_CLIENT_ID` | `release` environment variable | the Azure app registration's client id |
| `AZURE_TENANT_ID` | `release` environment variable | the Azure tenant id |
| `AZURE_SUBSCRIPTION_ID` | `release` environment variable | the subscription holding the signing account |
| `AZURE_SIGNING_ENDPOINT` | `release` environment variable | `https://ncus.codesigning.azure.net/` |
| `AZURE_SIGNING_ACCOUNT` | `release` environment variable | `mhuot-signing` |
| `AZURE_CERT_PROFILE` | `release` environment variable | the Public Trust certificate profile |

The Azure settings are variables, not secrets: none of them is secret, because the sign-in uses
GitHub's OIDC token. The workflow reads them as `vars.*` from the `release` environment, so
they are skipped if they are added as repository secrets instead.

### One-time setup for this repository

1. Add the six Mac repository secrets.
2. Create a GitHub environment named `release`. Under **Deployment branches and tags**, allow
   only `main` and tags matching `v*`.
3. Add the six Azure variables to that environment. The ids can be copied from another repo
   that already signs, e.g. `gh variable list --env release -R mhuot/loop-builder`.
4. In the Azure app registration (the one with the Certificate Profile Signer role on the
   signing account), add a federated credential for GitHub Actions with the subject
   `repo:mhuot/kvm-switcher-comet-x:environment:release`.

## Making a signed build

Run the CI workflow by hand on `main` (**Actions → CI → Run workflow**, leaving `release`
off), or push a `v*` tag. A run from any other branch fails within seconds with "The
deployment was rejected or didn't satisfy other protection rules", because the `release`
environment doesn't allow it.

A good run shows, in `macos-signed`, `status: Accepted` twice and "The staple and validate
action worked!" twice, then `source=Notarized Developer ID`. In `windows-signed` it shows
`Valid (CN=Mike Huot, ...)` for both exes, both installers and `unins000.exe`.

### Checking the downloaded files

On a Mac, after downloading the `.dmg` (so it carries the quarantine flag) and installing it:

```sh
xcrun stapler validate KVMSwitcher-<version>-macos.dmg
spctl -a -t open --context context:primary-signature -vv KVMSwitcher-<version>-macos.dmg
spctl -a -vv "/Applications/KVM Switcher.app"          # expect: accepted, source=Notarized Developer ID
codesign -dv --verbose=2 "/Applications/KVM Switcher.app"  # identity, runtime flag, timestamp
```

On Windows, in PowerShell:

```powershell
Get-AuthenticodeSignature .\KVMSwitcher-<version>-win-x64-setup.exe, .\KVMSwitcher-<version>-win-x64.exe |
    Format-Table Status, @{ Label = "Signer"; Expression = { $_.SignerCertificate.Subject } }, Path
```

SmartScreen can still warn about a new signed version until it builds up reputation. That is
expected, and it is not a signing failure.

## Signing locally

**macOS:** with the Developer ID certificate in your login keychain:

```sh
./macos/build-app.sh
export APPLE_SIGN_IDENTITY="Developer ID Application: <Name> (<TEAMID>)"
export APPLE_ID=<apple id> APPLE_TEAM_ID=<team id> APPLE_APP_PASSWORD=<app-specific password>
./macos/make-dmg.sh       # leave out the three notarytool settings to sign without notarizing
```

**Windows:** in PowerShell 7 on Windows, with the Azure CLI and Inno Setup installed, after
building the exes into `windows\publish\win-x64` and `windows\publish\win-arm64`:

```powershell
az login
windows\packaging\sign-setup.ps1 -Endpoint https://ncus.codesigning.azure.net/ -Account mhuot-signing -CertificateProfile <profile>
Get-ChildItem windows\publish -Recurse -Filter KVMSwitcher.exe | ForEach-Object { windows\packaging\sign.ps1 $_.FullName }
windows\packaging\make-installers.ps1 -Sign
```

## Pitfalls

| Symptom | Cause | Fix |
|---|---|---|
| Signing job fails in seconds with "deployment was rejected" | run from a branch the `release` environment doesn't allow | run from `main` or a `v*` tag |
| Azure steps skipped though the values exist | added as repository secrets | add them as variables of the `release` environment |
| Signing hangs before failing | the Azure credential chain tries every credential kind | `metadata.json` excludes all but the Azure CLI (`sign-setup.ps1` writes it) |
| Inno sign tool gets a literal `$f` or an empty path | PowerShell expanded `$q` and `$f` | keep the backticks in `make-installers.ps1` |
| A parameter named `$Profile` misbehaves in a signing script | it shadows PowerShell's automatic variable | the scripts use `-CertificateProfile` |
| App asks for internet on first launch, or Gatekeeper warns offline | only the `.dmg` was stapled | `make-dmg.sh` staples the app before building the `.dmg` |
