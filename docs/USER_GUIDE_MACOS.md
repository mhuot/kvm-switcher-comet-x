# KVM Switcher for macOS: User Guide

KVM Switcher puts your GL.iNet Comet X's port switch in the Mac menu bar. One click (or one
keystroke) moves the KVM to another computer, without opening the KVM's web page.

It's built for the **GL.iNet Comet X (GL-RM4PE)**, GL.iNet's 4-port remote KVM. KVM Switcher
is an independent project, not affiliated with GL.iNet.

Using Windows or Linux? See the [Windows guide](USER_GUIDE_WINDOWS.md) or the
[Linux guide](USER_GUIDE_LINUX.md).

## Contents

- [Install](#install)
- [First-time setup](#first-time-setup)
- [Switching ports](#switching-ports)
- [Naming your ports](#naming-your-ports)
- [The "wants to use your confidential information" Keychain prompt](#the-wants-to-use-your-confidential-information-keychain-prompt)
- [Troubleshooting](#troubleshooting)
- [Privacy and security](#privacy-and-security)
- [Uninstall](#uninstall)

## Install

1. Download `KVMSwitcher-<version>-macos.zip` from
   [Releases](https://github.com/mhuot/kvm-switcher-comet-x/releases/latest) and double-click it to unzip.
2. Drag **KVM Switcher** into your Applications folder, then open it.
3. The first time, macOS says it can't check the app for malicious software. KVM Switcher
   isn't signed with a paid Apple Developer ID, so macOS can't vouch for it. Click **Done**
   (not **Move to Trash**), open **System Settings → Privacy & Security**, scroll down to
   *"KVM Switcher" was blocked*, click **Open Anyway** and confirm. You only do this once
   per version.
4. Look for a small screen-over-an-arrow icon in the menu bar, near the clock. The app has no Dock icon
   and no main window. It lives only in the menu bar.

### Building it yourself instead

With Xcode or the Xcode command-line tools installed, open Terminal in the project folder and
run:

```sh
./macos/build-app.sh --install
```

This builds the app, copies **KVM Switcher** into your Applications folder and opens it.
There's no Gatekeeper warning for an app you built yourself.

## First-time setup

The Settings window opens by itself the first time. You can open it later from the menu
bar icon → **Settings…**.

1. **Address**: the KVM's network name or IP address, the same thing you type into your
   browser to reach the KVM (for example `glkvm.local`). You don't need `https://`.
2. **Admin password**: the password you use to log in to the KVM's web page.
3. Click **Test Connection**. You should see something like
   *"Connected. 4 ports, active: Workstation."*
4. Click **Save**.

On the first connection, macOS may show a Keychain prompt. That is expected, and
[the section below](#the-wants-to-use-your-confidential-information-keychain-prompt)
explains what to click.

### Optional settings

| Setting | What it does |
|---|---|
| **Port names** | Names to show for ports that have no name on the KVM. See [Naming your ports](#naming-your-ports). |
| **Menu bar shows** | What appears next to the menu bar icon: the active **Port name** (default), the **Port number**, or **Icon only**. Long names are shortened with "…". |
| **Global hotkeys** | Lets ⌃⌥⌘1 to ⌃⌥⌘4 switch ports from any app. On by default. |
| **Launch at login** | Starts KVM Switcher automatically when you log in to your Mac. |

## Switching ports

There are three ways to switch:

| How | Steps |
|---|---|
| **Click** | Click the menu bar icon, then click a port. |
| **Number keys** | Click the menu bar icon, then press **1**, **2**, **3** or **4**. |
| **Global hotkeys** | From any app, press **Control + Option + Command** and a number (⌃⌥⌘1 to ⌃⌥⌘4). |

### Reading the menu bar icon

| You see | It means |
|---|---|
| Screen with a number, e.g. **3**, and a port name | Connected. The KVM is on port 3. (Shows the port number or nothing beside the icon instead if you changed **Menu bar shows** in Settings.) |
| Screen with **…** | A switch is in progress, or the app hasn't connected yet. |
| Screen with **!** over a broken arrow | Something is wrong. Open the menu; the first line says what. |

In the menu, a checkmark marks the active port. "(no video)" after a port means the KVM
isn't receiving a picture from that computer (it may be asleep or turned off).

Other menu items:

- **Refresh** (⌘R): re-reads the KVM's state. The app also does this when you open the menu
  and every 30 seconds.
- **Open KVM Web UI** (⌘O): opens the KVM's web page in your browser.
- **Quit KVM Switcher** (⌘Q).

If you switch ports on the KVM itself or in its web page, the menu bar catches up the next
time it refreshes.

## Naming your ports

The easiest place to name ports is the KVM's own web page. KVM Switcher reads those names,
so they show up in the menu automatically.

If a port has no name there, KVM Switcher uses the name you typed under **Port names** in its
Settings. If both are blank, it shows "Port 1", "Port 2" and so on.

## The "wants to use your confidential information" Keychain prompt

You may see this:

> **KVM Switcher wants to use your confidential information stored in "net.mhuot.KVMSwitcher" in your keychain.**
> To allow this, enter the "login" keychain password.

### Why it appears

KVM Switcher doesn't keep your KVM password in a plain file. It stores it in the macOS
**Keychain**, the same secure storage Safari uses for saved passwords. The
"net.mhuot.KVMSwitcher" in the message is just the label of that saved KVM password.

macOS protects each Keychain item and only hands it to apps you've approved. When the app
needs the KVM password to log in, macOS checks with you first. That's this prompt.

### What to do

1. Type your **Mac login password** (the one you use to unlock your Mac). This is **not**
   the KVM password.
2. Click **Always Allow**.

| Button | Result |
|---|---|
| **Always Allow** | Recommended. KVM Switcher can read its saved password from now on without asking. |
| **Allow** | Works this once. You'll be asked again next time the app logs in to the KVM. |
| **Deny** | The app can't read the password, so it can't log in. The menu shows a login error. |

If you clicked **Deny** by mistake, open the menu and choose **Refresh**. The prompt comes
back, and you can choose **Always Allow**.

### Why it might come back

You should normally only see it once. It can come back when:

- **The app was rebuilt or updated.** The downloaded app has no Developer ID signature,
  so expect the prompt once after each new version. macOS ties "Always Allow" to the app's code
  signature. `macos/build-app.sh` signs with your Apple Development certificate when one is
  available, so the approval survives rebuilds. Without a certificate, the app gets a new
  temporary signature on every build, and macOS treats each build as a different app.
  Approve it once more after each such build.
- **You clicked Allow instead of Always Allow.**
- **You changed the KVM password in Settings.** Saving a new password can reset the approval.

### Is this safe?

Yes. The prompt comes from macOS itself, not from the app. KVM Switcher never sees your Mac
login password; macOS checks it and then decides whether to release the saved KVM password.
To see or remove the saved item, open the **Keychain Access** app (or **Passwords** on newer
macOS) and search for `net.mhuot.KVMSwitcher`.

## Troubleshooting

The first line of the menu shows the current problem, if there is one.

| Message or symptom | What to do |
|---|---|
| *Set the KVM address and password in Settings.* | Open **Settings…**, fill in both fields and click **Save**. |
| *Login rejected. Check the password in Settings.* | Re-enter the KVM admin password. Check that you can log in to the KVM's web page with it. |
| *2FA is enabled on the KVM; this app does not support it.* | KVM Switcher can't log in while two-factor login is turned on for the KVM. Turn it off in the KVM's web page to use the app. |
| *The KVM's certificate changed…* | See [Certificate warning](#certificate-warning) below. |
| *The request timed out* / *Could not connect to the server* | Check that the Mac and the KVM are on the same network, and that the address opens the KVM's web page in a browser. |
| Hotkeys do nothing | Check **Global hotkeys** is on in Settings. Another app may already use the same keys. |
| Keychain prompt every time | Click **Always Allow**, not **Allow**. See [Why it might come back](#why-it-might-come-back). |
| Port names missing | Name the ports in the KVM's web page, or under **Port names** in Settings, then choose **Refresh**. |

### Certificate warning

The KVM uses its own security certificate instead of one from a public authority (your
browser warns about this too the first time). KVM Switcher remembers the certificate the
first time it connects, and from then on only talks to a KVM showing that same certificate.
That stops another device on your network from pretending to be the KVM and collecting your
password.

The certificate changes legitimately after a factory reset, some firmware updates, or if you
point the app at a different KVM. In those cases, open **Settings…** → **Reset Pinned
Certificate**, then **Test Connection**. If you didn't change anything on the KVM, be
cautious before resetting.

## Privacy and security

- Your KVM password is stored only in your Mac's Keychain.
- The app talks only to the KVM address you enter, over HTTPS. Nothing is sent anywhere else.
- Port names and other preferences are stored in the app's standard macOS preferences.

## Uninstall

1. Choose **Quit KVM Switcher** from the menu.
2. If you turned on **Launch at login**, turn it off in Settings first, or remove KVM
   Switcher under **System Settings → General → Login Items** afterwards.
3. Drag **KVM Switcher** from Applications to the Trash.
4. To remove the saved password and preferences too, run in Terminal:

   ```sh
   security delete-generic-password -s net.mhuot.KVMSwitcher
   defaults delete net.mhuot.KVMSwitcher
   ```
