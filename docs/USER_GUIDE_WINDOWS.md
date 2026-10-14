# KVM Switcher for Windows 11: User Guide

KVM Switcher puts your GL.iNet Comet X's port switch in the Windows system tray. One click (or
one keystroke) moves the KVM to another computer, without opening the KVM's web page.

It's built for the **GL.iNet Comet X (GL-RM4PE)**, GL.iNet's 4-port remote KVM. KVM Switcher
is an independent project, not affiliated with GL.iNet.

Using a Mac or Linux? See the [macOS guide](USER_GUIDE_MACOS.md) or the
[Linux guide](USER_GUIDE_LINUX.md).

## Contents

- [Install](#install)
- [The "Windows protected your PC" warning](#the-windows-protected-your-pc-warning)
- [Keep the icon visible in the taskbar](#keep-the-icon-visible-in-the-taskbar)
- [First-time setup](#first-time-setup)
- [Switching ports](#switching-ports)
- [Naming your ports](#naming-your-ports)
- [Where your password is kept](#where-your-password-is-kept)
- [Troubleshooting](#troubleshooting)
- [Privacy and security](#privacy-and-security)
- [Uninstall](#uninstall)

## Install

KVM Switcher is a single file, `KVMSwitcher.exe`. It doesn't need an installer, admin rights,
or a separate .NET download.

1. Pick the right build for your PC:

   | Your PC | Use |
   |---|---|
   | Most Windows PCs (Intel or AMD processor) | `win-x64\KVMSwitcher.exe` |
   | Arm PCs, e.g. Surface Pro with Snapdragon / Copilot+ PCs | `win-arm64\KVMSwitcher.exe` |

   Not sure? Open **Settings → System → About** and look at **System type**. "x64-based
   processor" means x64; "ARM-based processor" means Arm64.
2. Create a folder for it, for example `C:\Users\<you>\AppData\Local\KVMSwitcher`, and copy
   `KVMSwitcher.exe` there. Use a permanent location: **Launch at login** points at wherever
   the file is.
3. Double-click `KVMSwitcher.exe`. If you see a blue "Windows protected your PC" box, read
   the next section.

## The "Windows protected your PC" warning

The first time you open the app, Windows may show:

> **Windows protected your PC**
> Microsoft Defender SmartScreen prevented an unrecognized app from starting.

### Why it appears

SmartScreen warns about any program that isn't signed with a paid code-signing certificate
and hasn't been downloaded by many people yet. KVM Switcher is a small personal tool, so
Windows has never seen it before. The warning doesn't mean Windows found anything harmful.

### What to do

1. Click **More info**.
2. Check that the app name is **KVMSwitcher.exe**.
3. Click **Run anyway**.

You only need to do this once per copy of the file. If you replace the `.exe` with a newer
build, Windows may ask once more.

If **Run anyway** doesn't appear, your organization may block unrecognized apps. Ask your IT
team.

## Keep the icon visible in the taskbar

Windows 11 hides new tray icons behind the **^** arrow next to the clock. To keep KVM
Switcher always visible:

1. Right-click the taskbar → **Taskbar settings**.
2. Expand **Other system tray icons**.
3. Turn on **KVM Switcher**.

You can also drag the icon from the **^** pop-up onto the taskbar.

## First-time setup

The Settings window opens by itself the first time. Later, open it from the tray icon →
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
| **Tray icon shows** | **Port number on icon** (default) draws the active port's number on the icon. **Plain icon** shows a monitor symbol instead. |
| **Global hotkeys** | Lets Ctrl+Alt+Win+1 to 4 switch ports from any app. On by default. |
| **Show a notification when a hotkey switches ports** | Pops up "Switched to …" after a hotkey switch, so you know it worked. |
| **Launch at login** | Starts KVM Switcher automatically when you sign in to Windows. |

## Switching ports

There are three ways to switch:

| How | Steps |
|---|---|
| **Click** | Click the tray icon (left or right click), then click a port. |
| **Number keys** | Click the tray icon, then press **1**, **2**, **3** or **4**. |
| **Global hotkeys** | From any app, press **Ctrl + Alt + Windows key** and a number. |

### Reading the tray icon

| You see | It means |
|---|---|
| Blue square with a number, e.g. **3** | Connected. The KVM is on port 3. |
| Grey square with **…** | A switch is in progress. |
| Grey square with **–** | The app hasn't connected yet. |
| Red square with **!** | Something is wrong. Hover over the icon or open the menu to see what. |

**Hover over the icon** to see the active port's name, for example "KVM: Media PC".
(Unlike the Mac menu bar, the Windows tray can't show text beside an icon.)

In the menu, a checkmark marks the active port. "(no video)" after a port means the KVM
isn't receiving a picture from that computer (it may be asleep or turned off).

Other menu items:

- **Refresh**: re-reads the KVM's state. The app also does this when you open the menu and
  every 30 seconds.
- **Open KVM Web UI**: opens the KVM's web page in your browser.
- **Quit KVM Switcher**.

## Naming your ports

The easiest place to name ports is the KVM's own web page. KVM Switcher reads those names,
so they show up in the menu automatically.

If a port has no name there, KVM Switcher uses the name you typed under **Port names** in its
Settings. If both are blank, it shows "Port 1", "Port 2" and so on.

## Where your password is kept

KVM Switcher stores your KVM password in **Windows Credential Manager**, the same secure
store Windows uses for saved network and app passwords. It is never written to a plain file.

Unlike the Mac version, Windows doesn't ask you to approve access: Credential Manager items
belong to your Windows account, and apps you run can read them without a prompt.

To see or remove it, open **Control Panel → User Accounts → Credential Manager → Windows
Credentials** and look for **KVMSwitcher/glkvm-admin** under *Generic Credentials*.

## Troubleshooting

The icon's tooltip and the first line of the menu show the current problem, if there is one.

| Message or symptom | What to do |
|---|---|
| *Set the KVM address and password in Settings.* | Open **Settings…**, fill in both fields and click **Save**. |
| *Login rejected. Check the password in Settings.* | Re-enter the KVM admin password. Check that you can log in to the KVM's web page with it. |
| *2FA is enabled on the KVM; this app does not support it.* | KVM Switcher can't log in while two-factor login is turned on for the KVM. Turn it off in the KVM's web page to use the app. |
| *The KVM's certificate changed…* | See [Certificate warning](#certificate-warning) below. |
| *Could not reach the KVM…* / *The KVM did not respond in time.* | Check that the PC and the KVM are on the same network, and that the address opens the KVM's web page in a browser. If you use `glkvm.local`, try the KVM's IP address instead. |
| Menu says *Hotkey Ctrl+Alt+Win+… in use by another app* | Another program already uses that key combination. Close it, then **Save** in Settings again to retry. |
| Hotkeys do nothing | Check **Global hotkeys** is on in Settings. Some apps running as administrator block hotkeys while they're in front. |
| Can't find the icon | Look under the **^** arrow by the clock, then follow [Keep the icon visible](#keep-the-icon-visible-in-the-taskbar). |
| Nothing happens when opening the `.exe` | It's probably already running. Only one copy runs at a time; look in the tray. |
| No "Switched to…" pop-up | Check the notification setting in KVM Switcher, and that **Do not disturb** is off in Windows. |
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

- Your KVM password is stored only in Windows Credential Manager.
- The app talks only to the KVM address you enter, over HTTPS. Nothing is sent anywhere else.
- Other settings are kept in `%APPDATA%\KVMSwitcher\settings.json`.

## Uninstall

1. Right-click the tray icon → **Quit KVM Switcher**. If you turned on **Launch at login**,
   turn it off in Settings first.
2. Delete `KVMSwitcher.exe`.
3. To remove saved settings and the password too:
   - Delete the folder `%APPDATA%\KVMSwitcher` (paste that into File Explorer's address bar).
   - In Credential Manager, remove **KVMSwitcher/glkvm-admin**.
