# KVM Switcher for Linux: User Guide

KVM Switcher puts your GL.iNet Comet X's port switch in your desktop's panel (the top or bottom
bar). One click, or one keyboard shortcut, moves the KVM to another computer without opening
the KVM's web page. It also works from the command line, including on machines with no
desktop at all.

It's built for the **GL.iNet Comet X (GL-RM4PE)**, GL.iNet's 4-port remote KVM. KVM Switcher
is an independent project, not affiliated with GL.iNet.

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
- [Headless machines and scripts](#headless-machines-and-scripts)
- [Troubleshooting](#troubleshooting)
- [Privacy and security](#privacy-and-security)
- [Uninstall](#uninstall)

## What you need

KVM Switcher is a single program with no other software to install. It's tested on Ubuntu
22.04 and 24.04, Debian 12 and 13, Fedora, Arch Linux and openSUSE Tumbleweed, on both
64-bit PCs (x86_64) and 64-bit Arm (aarch64, such as a Raspberry Pi running a 64-bit OS).

The panel icon works on any desktop that shows "StatusNotifierItem" (also called
AppIndicator) icons: KDE Plasma, Cinnamon, MATE, Xfce, Budgie, LXQt, waybar, and GNOME with
an extension (see below). It works on both Wayland and X11.

## Install

1. Open the [Releases page](https://github.com/mhuot/kvm-switcher-comet-x/releases/latest)
   and download the file for your distribution and processor. Not sure which processor you
   have? Run `uname -m` in a terminal: `x86_64` means **amd64** (most PCs), `aarch64` means
   **arm64**.

   | Distribution | Download | Install with |
   |---|---|---|
   | Ubuntu, Debian, Mint, Pop!_OS, Raspberry Pi OS | `kvm-switcher_<version>-1_amd64.deb` or `_arm64.deb` | `sudo apt install ./kvm-switcher_*.deb` |
   | Fedora | `kvm-switcher-<version>-1.x86_64.rpm` or `.aarch64.rpm` | `sudo dnf install ./kvm-switcher-*.rpm` |
   | openSUSE | the same `.rpm` | `sudo zypper install --allow-unsigned-rpm ./kvm-switcher-*.rpm` |
   | Arch, Manjaro, EndeavourOS | `kvm-switcher-<version>-1-x86_64.pkg.tar.zst` or `-aarch64` | `sudo pacman -U ./kvm-switcher-*.pkg.tar.zst` |
   | Anything else, or no `sudo` | `kvm-switcher-<version>-linux-amd64.tar.gz` or `-arm64` | see below |

   Run the install command in the folder you downloaded to (usually `~/Downloads`). The
   packages aren't signed. To check a download, also get `SHA256SUMS.txt` and run
   `sha256sum -c --ignore-missing SHA256SUMS.txt`.

   **Without a package (any distribution, no `sudo`):** unpack the tarball and run its
   installer, which installs for your user only:

   ```sh
   tar xzf kvm-switcher-*-linux-*.tar.gz
   cd kvm-switcher-*-linux-*/
   ./install.sh
   ```

2. Open **KVM Switcher** from your applications menu.

To update, download and install the new version the same way.

## GNOME: make the panel icon appear

Plain GNOME doesn't show panel icons from apps. You need the
**AppIndicator and KStatusNotifierItem Support** extension:

| Distribution | What to do |
|---|---|
| Ubuntu | Nothing. It's already installed and on. |
| Fedora | `sudo dnf install gnome-shell-extension-appindicator`, log out and back in, then turn it on in the **Extensions** app. |
| Others | Install "AppIndicator and KStatusNotifierItem Support" from your package manager or extensions.gnome.org, then turn it on in **Extensions**. |

KDE Plasma, Cinnamon, MATE, Xfce and Budgie show the icon without anything extra. If the
icon doesn't appear, KVM Switcher shows a notification saying so.

## First-time setup

Settings open by themselves the first time. Later, open them from the panel icon →
**Settings…**, or run `kvm-switcher setup` in a terminal.

On most desktops the settings appear as small windows (KVM Switcher uses `zenity` on GNOME
and most other desktops, and `kdialog` on KDE). If neither is installed, **Settings…** opens
the same questions in a terminal window instead.

1. **Address**: the KVM's network name or IP address, the same thing you type into your
   browser to reach the KVM (for example `glkvm.local`). You don't need `https://`.
2. **Admin password**: the password you use to log in to the KVM's web page.
3. KVM Switcher connects and shows something like
   *"Connected. 4 ports, active: Workstation."*

### Other settings

Open **Settings…** again to see every setting in one list. Pick one and choose **Change…**.

| Setting | What it does |
|---|---|
| **Test connection** | Connects with the saved address and password and shows the result. |
| **Port names** | Names to show for ports that have no name on the KVM. See [Naming your ports](#naming-your-ports). |
| **Panel shows** | What appears beside the panel icon: the active **Port name** (default), the **Port number**, or **Icon only**. Long names are shortened with "…". |
| **Start when I log in** | Starts KVM Switcher automatically after you log in. |
| **Reset pinned certificate** | See [Certificate warning](#certificate-warning). |

Changes are saved as soon as you make them. Some desktops (KDE Plasma, for example) show
only the icon and never the text beside it. Hover over the icon or open its menu to see the
active port there.

## Switching ports

| How | Steps |
|---|---|
| **Click** | Click the panel icon, then click a port. |
| **Number keys** | Click the panel icon, then press **1**, **2**, **3** or **4**. |
| **Keyboard shortcut** | Set one up once, as described in [Keyboard shortcuts](#keyboard-shortcuts). |

### Reading the panel

| You see | It means |
|---|---|
| Screen with a number, e.g. **3**, over a ⟷ arrow | Connected. The KVM is on port 3. The port name shows beside it unless **Panel shows** is set to **Icon only**. |
| Screen with **…** | A switch is in progress, or the app hasn't connected yet. |
| Screen with **!** over a broken arrow | Something is wrong. Open the menu; the first line says what. |

In the menu, the active port is selected. "(no video)" after a port means the KVM isn't
receiving a picture from that computer (it may be asleep or turned off).

Other menu items:

- **Refresh**: re-reads the KVM's state. The app also does this every 15 seconds and each
  time you open the menu.
- **Open KVM Web UI**: opens the KVM's web page in your browser.
- **Settings…**
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

If you installed from the tarball and the shortcut does nothing, use the full path instead,
for example `/home/<you>/.local/bin/kvm-switcher switch 1` (some desktops don't read
`~/.local/bin`).

When KVM Switcher is running in the panel, the shortcut hands the request to it, so the panel
updates immediately. If it isn't running, the command switches the KVM on its own.

## Naming your ports

The easiest place to name ports is the KVM's own web page. KVM Switcher reads those names,
so they show up in the menu automatically.

If a port has no name there, KVM Switcher uses the name you set under **Port names** in its
settings. If both are blank, it shows "Port 1", "Port 2" and so on.

## Where your password is kept

KVM Switcher stores your KVM password in your desktop's **keyring**, the same secure store
your browser and Wi-Fi settings use: GNOME Keyring on GNOME, Cinnamon, MATE and Xfce, or
KWallet on KDE Plasma (KeePassXC works too, with its Secret Service option on).

The keyring normally unlocks when you log in. If your keyring is locked (for example after
an automatic login), your desktop may ask for your **keyring password** (usually your login
password) the first time KVM Switcher reads it. Enter it and choose to unlock.

To see or remove it, open **Passwords and Keys** (Seahorse) on GNOME, or **KWalletManager**
on KDE, and look for **KVM Switcher (GL.iNet KVM admin password)**.

On machines without a keyring, see [Headless machines and scripts](#headless-machines-and-scripts).

## Using it from the terminal

| Command | What it does |
|---|---|
| `kvm-switcher` | Starts the panel icon |
| `kvm-switcher switch 3` | Switches to port 3 |
| `kvm-switcher status` | Prints the active port's name |
| `kvm-switcher list` | Lists all ports; `*` marks the active one |
| `kvm-switcher setup` | Changes the settings, asking questions in the terminal |
| `kvm-switcher self-test` | Checks that the panel icon and keyring work on this machine |
| `kvm-switcher version` | Shows the version |

The commands use the same settings and password as the panel app.

## Headless machines and scripts

KVM Switcher's commands work without a desktop, for example on a server you reach over
SSH. Install the `.deb`, `.rpm` or Arch package as above (they need nothing else), then run:

```sh
kvm-switcher setup
```

It asks for the address and password in the terminal. With no keyring on the machine, it
offers to save the password in `~/.config/kvm-switcher/password` instead. Only your user
account can read that file, but it isn't encrypted.

To set up without questions, for example from a provisioning script:

```sh
echo "$KVM_PASSWORD" | kvm-switcher setup --address glkvm.local --password-stdin \
    --save-password-to-file --test
```

Leave out `--save-password-to-file` to use the keyring when there is one. Run
`kvm-switcher setup --help` for every option.

The password can also come from somewhere else. KVM Switcher checks these in order and uses
the first one it finds:

| Source | Example |
|---|---|
| `KVM_SWITCHER_PASSWORD` environment variable | `KVM_SWITCHER_PASSWORD=… kvm-switcher switch 2` |
| A file named by `KVM_SWITCHER_PASSWORD_FILE` | `KVM_SWITCHER_PASSWORD_FILE=/run/secrets/kvm kvm-switcher list` |
| A systemd credential named `kvm-switcher-password` | `LoadCredential=kvm-switcher-password:/etc/kvm-switcher/password` in a service |
| The desktop keyring | Saved by **Settings…** or `kvm-switcher setup` |
| `~/.config/kvm-switcher/password` (mode 600) | Saved by `kvm-switcher setup` when there's no keyring |

Files are read up to the first line break. KVM Switcher ignores
`~/.config/kvm-switcher/password` if other users can read it.

## Troubleshooting

Start with `kvm-switcher self-test`. It checks the panel icon, the menu and the keyring
without needing the KVM, and says what's missing.

| Message or symptom | What to do |
|---|---|
| No panel icon on GNOME | Install and turn on the AppIndicator extension. See [GNOME](#gnome-make-the-panel-icon-appear). |
| *…your desktop isn't showing panel icons* | Same as above, or use a desktop that shows StatusNotifierItem icons. The commands and shortcuts still work. |
| *Set the KVM address and password in Settings.* | Open **Settings…** or run `kvm-switcher setup`. |
| *Could not save the password: … keyring …* | No keyring is running. Log out and back in, install `gnome-keyring` (or use KWallet on KDE), or accept the password file when setup offers it. |
| *Login rejected. Check the password in Settings.* | Re-enter the KVM admin password. Check that you can log in to the KVM's web page with it. |
| *2FA is enabled on the KVM; this app does not support it.* | KVM Switcher can't log in while two-factor login is turned on for the KVM. Turn it off in the KVM's web page to use the app. |
| *The KVM's certificate changed…* | See [Certificate warning](#certificate-warning) below. |
| *Could not reach the KVM…* / *The KVM did not respond in time.* | Check that the computer and the KVM are on the same network and that the address opens the KVM's web page in a browser. If `glkvm.local` doesn't work, use the KVM's IP address (`.local` names need Avahi/mDNS). |
| **Settings…** opens a terminal instead of windows | Install `zenity` (or `kdialog` on KDE) for settings windows. |
| Shortcut does nothing | Use the full path to `kvm-switcher` in the shortcut. Run the command in a terminal to see any error. |
| `kvm-switcher: command not found` after a tarball install | Add `~/.local/bin` to your `PATH`, or use the full path. |
| Port names missing | Name the ports in the KVM's web page, or under **Port names** in settings, then choose **Refresh**. |

### Certificate warning

The KVM uses its own security certificate instead of one from a public authority (your
browser warns about this too the first time). KVM Switcher remembers the certificate the
first time it connects, and from then on only talks to a KVM showing that same certificate.
That stops another device on your network from pretending to be the KVM and collecting your
password.

The certificate changes legitimately after a factory reset or some firmware updates. In
those cases, open **Settings…** → **Reset pinned certificate**, then **Test connection**. If
you didn't change anything on the KVM, be cautious before resetting. Changing the
**Address** to a different KVM resets it automatically.

## Privacy and security

- Your KVM password is stored only in your desktop keyring, or in the place you chose from
  [the list above](#headless-machines-and-scripts).
- The app talks only to the KVM address you enter, over HTTPS. Nothing is sent anywhere else.
- Other settings are kept in `~/.config/kvm-switcher/settings.json`.

## Uninstall

| Installed with | Remove with |
|---|---|
| `.deb` | `sudo apt remove kvm-switcher` |
| `.rpm` on Fedora | `sudo dnf remove kvm-switcher` |
| `.rpm` on openSUSE | `sudo zypper remove kvm-switcher` |
| Arch package | `sudo pacman -R kvm-switcher` |
| Tarball | `./install.sh --uninstall` from the unpacked folder |

Then, to remove everything else too:

- Turn off **Start when I log in** first, or delete
  `~/.config/autostart/net.mhuot.KVMSwitcher.desktop`.
- Delete `~/.config/kvm-switcher` (settings, and the password file if you used one).
- Remove **KVM Switcher (GL.iNet KVM admin password)** from Passwords and Keys or
  KWalletManager.
- Remove any keyboard shortcuts you added.
