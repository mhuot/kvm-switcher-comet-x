# KVM Switcher for Linux: User Guide

KVM Switcher puts your GL.iNet KVM's port switch in your desktop's panel (the top or bottom
bar). One click, or one keyboard shortcut, moves the KVM to another computer without opening
the KVM's web page.

Using a Mac or Windows PC? See the [macOS guide](USER_GUIDE_MACOS.md) or the
[Windows guide](USER_GUIDE_WINDOWS.md).

## Contents

- [What you need](#what-you-need)
- [Install](#install)
- [GNOME: make the panel icon appear](#gnome-make-the-panel-icon-appear)
- [First-time setup](#first-time-setup)
- [Switching ports](#switching-ports)
- [Keyboard shortcuts](#keyboard-shortcuts)
- [Naming your ports](#naming-your-ports)
- [Where your password is kept](#where-your-password-is-kept)
- [Using it from the terminal](#using-it-from-the-terminal)
- [Troubleshooting](#troubleshooting)
- [Privacy and security](#privacy-and-security)
- [Uninstall](#uninstall)

## What you need

KVM Switcher is tested on Ubuntu 22.04 and 24.04, Debian 12 and 13, Fedora, Arch Linux and
openSUSE Tumbleweed. It works with any desktop that shows "AppIndicator" panel icons: KDE
Plasma, Cinnamon, MATE, Xfce, Budgie, and GNOME with an extension (see below). It runs on both
Wayland and X11.

## Install

1. Install the system packages for your distribution:

   | Distribution | Command |
   |---|---|
   | Ubuntu, Debian, Mint, Pop!_OS | `sudo apt install python3-venv python3-gi gir1.2-gtk-3.0 gir1.2-ayatanaappindicator3-0.1 gir1.2-secret-1` |
   | Fedora | `sudo dnf install python3-gobject gtk3 libayatana-appindicator-gtk3 libsecret` |
   | Arch, Manjaro, EndeavourOS | `sudo pacman -S python-gobject gtk3 libayatana-appindicator libsecret` |
   | openSUSE | `sudo zypper install python3-gobject python3-gobject-Gdk typelib-1_0-Gtk-3_0 typelib-1_0-AyatanaAppIndicator3-0_1 typelib-1_0-Secret-1` |

2. From the project folder, run:

   ```sh
   ./linux/install.sh
   ```

   This installs KVM Switcher for your user only (no `sudo`), adds **KVM Switcher** to your
   applications menu, and adds the `kvm-switcher` command. If a package from step 1 is
   missing, the script says which one.
3. Open **KVM Switcher** from your applications menu.

To update later, get the new version and run `./linux/install.sh` again.

## GNOME: make the panel icon appear

Plain GNOME doesn't show panel icons from apps. You need the
**AppIndicator and KStatusNotifierItem Support** extension:

| Distribution | What to do |
|---|---|
| Ubuntu | Nothing. It's already installed and on. |
| Fedora | `sudo dnf install gnome-shell-extension-appindicator`, log out and back in, then turn it on in the **Extensions** app. |
| Others | Install "AppIndicator and KStatusNotifierItem Support" from your package manager or extensions.gnome.org, then turn it on in **Extensions**. |

KDE Plasma, Cinnamon, MATE, Xfce and Budgie show the icon without anything extra.

## First-time setup

The Settings window opens by itself the first time. Later, open it from the panel icon →
**Settings…**.

1. **Address**: the KVM's network name or IP address, the same thing you type into your
   browser to reach the KVM (for example `glkvm.local`). You don't need `https://`.
2. **Admin password**: the password you use to log in to the KVM's web page.
3. Click **Test Connection**. You should see something like
   *"Connected. 4 ports, active: Workstation."*
4. Click **Save**, then **Close**.

### Optional settings

| Setting | What it does |
|---|---|
| **Port names** | Names to show for ports that have no name on the KVM. See [Naming your ports](#naming-your-ports). |
| **Panel shows** | What appears beside the panel icon: the active **Port name** (default), the **Port number**, or **Icon only**. Long names are shortened with "…". |
| **Start KVM Switcher when I log in** | Starts it automatically after you log in. |

Some desktops (KDE Plasma, for example) show only the icon and never the text beside it.
Hover over the icon or open its menu to see the active port there.

## Switching ports

| How | Steps |
|---|---|
| **Click** | Click the panel icon, then click a port. |
| **Number keys** | Click the panel icon, then press **1**, **2**, **3** or **4**. |
| **Keyboard shortcut** | Set one up once, as described in [Keyboard shortcuts](#keyboard-shortcuts). |

### Reading the panel

| You see | It means |
|---|---|
| Screen icon and a port name, e.g. **Media PC** | Connected. The KVM is on that port. |
| Screen icon and **…** | A switch is in progress. |
| Screen icon alone | Not connected yet, or **Panel shows** is set to **Icon only**. |
| Warning icon | Something is wrong. Open the menu; the first line says what. |

In the menu, the active port is selected. "(no video)" after a port means the KVM isn't
receiving a picture from that computer (it may be asleep or turned off).

Other menu items:

- **Refresh**: re-reads the KVM's state. The app also does this every 15 seconds.
- **Open KVM Web UI**: opens the KVM's web page in your browser.
- **Quit KVM Switcher**.

## Keyboard shortcuts

On Linux, apps can't safely grab keyboard shortcuts for themselves (Wayland doesn't allow
it). Instead, you add the shortcuts in your desktop's own settings, and they run a short
command. This works on every desktop, on both Wayland and X11.

Make one shortcut per port, with these commands:

| Shortcut (suggested) | Command |
|---|---|
| Ctrl+Alt+Super+1 | `kvm-switcher switch 1` |
| Ctrl+Alt+Super+2 | `kvm-switcher switch 2` |
| Ctrl+Alt+Super+3 | `kvm-switcher switch 3` |
| Ctrl+Alt+Super+4 | `kvm-switcher switch 4` |

Where to add them:

- **GNOME / Ubuntu**: Settings → Keyboard → View and Customize Shortcuts → Custom Shortcuts → **+**
- **KDE Plasma**: System Settings → Keyboard → Shortcuts → Add New → Command or Script
- **Cinnamon**: System Settings → Keyboard → Shortcuts → Custom Shortcuts → Add custom shortcut
- **Xfce**: Settings → Keyboard → Application Shortcuts → **+ Add**

If the shortcut does nothing, use the full path instead of `kvm-switcher`, for example
`/home/<you>/.local/bin/kvm-switcher switch 1` (some desktops don't read `~/.local/bin`).

When KVM Switcher is running in the panel, the shortcut hands the request to it, so the panel
updates immediately. If it isn't running, the command switches the KVM on its own.

## Naming your ports

The easiest place to name ports is the KVM's own web page. KVM Switcher reads those names,
so they show up in the menu automatically.

If a port has no name there, KVM Switcher uses the name you typed under **Port names** in its
Settings. If both are blank, it shows "Port 1", "Port 2" and so on.

## Where your password is kept

KVM Switcher stores your KVM password in your desktop's **keyring**, the same secure store
your browser and Wi-Fi settings use: GNOME Keyring on GNOME, Cinnamon, MATE and Xfce, or
KWallet on KDE Plasma. It is never written to a plain file.

The keyring normally unlocks when you log in. If your keyring is locked (for example after
an automatic login), your desktop may ask for your **keyring password** (usually your login
password) the first time KVM Switcher reads it. Enter it and choose to unlock.

To see or remove it, open **Passwords and Keys** (Seahorse) on GNOME, or **KWalletManager**
on KDE, and look for **KVM Switcher (GL.iNet KVM admin password)**.

For scripts or machines without a keyring, you can set the password in the
`KVM_SWITCHER_PASSWORD` environment variable instead. It takes priority over the keyring.

## Using it from the terminal

| Command | What it does |
|---|---|
| `kvm-switcher` | Starts the panel icon |
| `kvm-switcher switch 3` | Switches to port 3 |
| `kvm-switcher status` | Prints the active port's name |
| `kvm-switcher list` | Lists all ports; `*` marks the active one |
| `kvm-switcher --version` | Shows the version |

The commands use the same settings and password as the panel app, so set it up in Settings
first.

## Troubleshooting

| Message or symptom | What to do |
|---|---|
| No panel icon on GNOME | Install and turn on the AppIndicator extension. See [GNOME](#gnome-make-the-panel-icon-appear). |
| *KVM Switcher needs AppIndicator support…* | Install the AppIndicator package for your distribution from [Install](#install), step 1. |
| *Set the KVM address and password in Settings.* | Open **Settings…**, fill in both fields and click **Save**. |
| *Could not save: … keyring …* | No keyring service is running. Log out and back in, install `gnome-keyring` (or use KWallet on KDE), or use `KVM_SWITCHER_PASSWORD`. |
| *Login rejected. Check the password in Settings.* | Re-enter the KVM admin password. Check that you can log in to the KVM's web page with it. |
| *2FA is enabled on the KVM; this app does not support it.* | KVM Switcher can't log in while two-factor login is turned on for the KVM. Turn it off in the KVM's web page to use the app. |
| *The KVM's certificate changed…* | See [Certificate warning](#certificate-warning) below. |
| *Could not reach the KVM…* / *The KVM did not respond in time.* | Check that the computer and the KVM are on the same network and that the address opens the KVM's web page in a browser. If `glkvm.local` doesn't work, use the KVM's IP address (`.local` names need Avahi/mDNS). |
| Shortcut does nothing | Use the full path to `kvm-switcher` in the shortcut. Run the command in a terminal to see any error. |
| `kvm-switcher: command not found` | Add `~/.local/bin` to your `PATH`, or use the full path. |
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

- Your KVM password is stored only in your desktop keyring (or the environment variable, if
  you choose to use it).
- The app talks only to the KVM address you enter, over HTTPS. Nothing is sent anywhere else.
- Other settings are kept in `~/.config/kvm-switcher/settings.json`.

## Uninstall

```sh
./linux/install.sh --uninstall
```

This removes the app, the menu entry, the `kvm-switcher` command and the start-at-login
entry. To remove everything else too:

- Delete `~/.config/kvm-switcher`.
- Remove **KVM Switcher (GL.iNet KVM admin password)** from Passwords and Keys or
  KWalletManager.
- Remove any keyboard shortcuts you added.
