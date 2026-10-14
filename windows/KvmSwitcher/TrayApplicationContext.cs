using System.Diagnostics;
using System.Reflection;

namespace KvmSwitcher;

/// <summary>Owns the tray icon, the port menu and the settings window.</summary>
internal sealed class TrayApplicationContext : ApplicationContext
{
    private const int BackgroundRefreshMilliseconds = 30_000;
    private const int DefaultPortCount = 4;
    private const int MaximumTooltipLength = 127;

    private readonly AppSettings settings = AppSettings.Load();
    private readonly KvmClient kvmClient;
    private readonly NotifyIcon notifyIcon;
    private readonly ContextMenuStrip portMenu = new();
    private readonly System.Windows.Forms.Timer refreshTimer = new() { Interval = BackgroundRefreshMilliseconds };
    private readonly List<ToolStripMenuItem> portMenuItems = new();
    private GlobalHotKeys? globalHotKeys;
    private SettingsForm? settingsForm;
    private Icon? currentTrayIcon;

    private KvmSwitchState? switchState;
    private string? lastErrorMessage;
    private string? hotKeyWarning;
    private bool switchInProgress;
    private bool refreshInProgress;

    public TrayApplicationContext()
    {
        kvmClient = new KvmClient(settings);
        notifyIcon = new NotifyIcon { ContextMenuStrip = portMenu, Visible = true };
        notifyIcon.MouseClick += (_, mouseEvent) =>
        {
            if (mouseEvent.Button == MouseButtons.Left)
            {
                ShowMenuAtCursor();
            }
        };
        portMenu.Opening += async (_, _) => await RefreshStateAsync();
        refreshTimer.Tick += async (_, _) => await RefreshStateAsync();

        UpdateTrayIcon();
        RebuildMenu();
        ApplyHotKeySetting();
        refreshTimer.Start();
        _ = RefreshStateAsync();

        if (string.IsNullOrEmpty(settings.Host) || string.IsNullOrEmpty(CredentialStore.ReadPassword()))
        {
            ShowSettings();
        }
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing)
        {
            refreshTimer.Dispose();
            globalHotKeys?.Dispose();
            notifyIcon.Visible = false;
            notifyIcon.Dispose();
            portMenu.Dispose();
            currentTrayIcon?.Dispose();
            kvmClient.Dispose();
        }

        base.Dispose(disposing);
    }

    // ---- State ----

    private async Task RefreshStateAsync()
    {
        if (refreshInProgress)
        {
            return;
        }

        refreshInProgress = true;
        try
        {
            switchState = await kvmClient.FetchSwitchStateAsync();
            lastErrorMessage = null;
        }
        catch (KvmException kvmError)
        {
            lastErrorMessage = kvmError.Message;
        }
        finally
        {
            refreshInProgress = false;
        }

        UpdateTrayIcon();
        RebuildMenu();
    }

    private async Task SwitchToPortAsync(int portIndex, bool fromHotKey)
    {
        if (switchInProgress || (switchState is not null && portIndex >= switchState.Ports.Count))
        {
            return;
        }

        switchInProgress = true;
        UpdateTrayIcon();
        try
        {
            await kvmClient.SetActivePortAsync(portIndex);
            lastErrorMessage = null;
            // The switch reports "synced: false" briefly; give it a moment before reading back.
            await Task.Delay(400);
        }
        catch (KvmException kvmError)
        {
            lastErrorMessage = kvmError.Message;
            System.Media.SystemSounds.Beep.Play();
        }
        finally
        {
            switchInProgress = false;
        }

        await RefreshStateAsync();

        if (fromHotKey && settings.ShowSwitchNotifications)
        {
            string notificationText = lastErrorMessage ?? $"Switched to {ActivePortName() ?? $"Port {portIndex + 1}"}";
            notifyIcon.ShowBalloonTip(
                2000, "KVM Switcher", notificationText, lastErrorMessage is null ? ToolTipIcon.None : ToolTipIcon.Warning);
        }
    }

    private string? ActivePortName()
    {
        if (switchState is null || switchState.ActivePortIndex >= switchState.Ports.Count)
        {
            return null;
        }

        return switchState.Ports[switchState.ActivePortIndex].DisplayName(settings);
    }

    // ---- Tray icon ----

    private void UpdateTrayIcon()
    {
        TrayIconRenderer.IconState iconState;
        if (switchInProgress)
        {
            iconState = TrayIconRenderer.IconState.Switching;
        }
        else if (lastErrorMessage is not null)
        {
            iconState = TrayIconRenderer.IconState.Error;
        }
        else if (switchState is null)
        {
            iconState = TrayIconRenderer.IconState.NotConnected;
        }
        else
        {
            iconState = TrayIconRenderer.IconState.Connected;
        }

        string badgeText = switchState is null ? "" : $"{switchState.ActivePortIndex + 1}";
        Icon newTrayIcon = TrayIconRenderer.Render(
            iconState, badgeText, settings.TrayIconStyle == TrayIconStyle.PlainIcon);
        notifyIcon.Icon = newTrayIcon;
        currentTrayIcon?.Dispose();
        currentTrayIcon = newTrayIcon;

        // Hovering the tray icon is how Windows users see the active port's name.
        string tooltipText = lastErrorMessage ?? (ActivePortName() is { } activeName
            ? $"KVM: {activeName}"
            : "KVM Switcher");
        notifyIcon.Text = tooltipText.Length > MaximumTooltipLength
            ? tooltipText[..(MaximumTooltipLength - 1)] + "…"
            : tooltipText;
    }

    // ---- Menu ----

    private void RebuildMenu()
    {
        IReadOnlyList<KvmPort> ports = switchState?.Ports ?? Enumerable.Range(0, DefaultPortCount)
            .Select(portIndex => new KvmPort(portIndex, "", "", true))
            .ToList();

        // Rebuilding a menu while it is on screen makes it flicker; update the port rows in place instead.
        if (portMenu.Visible && portMenuItems.Count == ports.Count)
        {
            foreach (KvmPort port in ports)
            {
                ApplyPortItemState(portMenuItems[port.PortIndex], port);
            }

            return;
        }

        portMenu.Items.Clear();
        portMenuItems.Clear();

        if (lastErrorMessage is not null)
        {
            portMenu.Items.Add(new ToolStripMenuItem(lastErrorMessage) { Enabled = false });
            portMenu.Items.Add(new ToolStripSeparator());
        }

        foreach (KvmPort port in ports)
        {
            var portItem = new ToolStripMenuItem();
            int selectedPortIndex = port.PortIndex;
            portItem.Click += async (_, _) => await SwitchToPortAsync(selectedPortIndex, fromHotKey: false);
            ApplyPortItemState(portItem, port);
            portMenuItems.Add(portItem);
            portMenu.Items.Add(portItem);
        }

        portMenu.Items.Add(new ToolStripSeparator());
        if (settings.GlobalHotKeysEnabled)
        {
            string hotKeyHint = hotKeyWarning
                ?? $"Hotkeys: {GlobalHotKeys.ModifierDescription}+1 – {ports.Count}";
            portMenu.Items.Add(new ToolStripMenuItem(hotKeyHint) { Enabled = false });
        }

        portMenu.Items.Add("&Refresh", null, async (_, _) => await RefreshStateAsync());
        portMenu.Items.Add("&Open KVM Web UI", null, (_, _) => OpenWebUI());
        portMenu.Items.Add("&Settings…", null, (_, _) => ShowSettings());
        portMenu.Items.Add(new ToolStripSeparator());
        portMenu.Items.Add("&Quit KVM Switcher", null, (_, _) => ExitThread());
    }

    private void ApplyPortItemState(ToolStripMenuItem portItem, KvmPort port)
    {
        // "&1" makes the number key select the port while the menu is open; escape "&" in names.
        string escapedName = port.DisplayName(settings).Replace("&", "&&");
        string mnemonic = port.PortIndex < 9 ? $"&{port.PortIndex + 1}   " : "";
        portItem.Text = mnemonic + escapedName + (port.HasVideoLink ? "" : "  (no video)");
        portItem.Checked = port.PortIndex == switchState?.ActivePortIndex;
        portItem.Enabled = switchState is not null;
    }

    private void ShowMenuAtCursor()
    {
        // NotifyIcon only opens its menu on right-click; reuse its own private method for left-click
        // so the menu is positioned and dismissed exactly like the right-click menu.
        MethodInfo? showContextMenu = typeof(NotifyIcon).GetMethod(
            "ShowContextMenu", BindingFlags.Instance | BindingFlags.NonPublic);
        if (showContextMenu is not null)
        {
            showContextMenu.Invoke(notifyIcon, null);
        }
        else
        {
            portMenu.Show(Cursor.Position);
        }
    }

    private void OpenWebUI()
    {
        if (string.IsNullOrEmpty(settings.Host))
        {
            return;
        }

        Process.Start(new ProcessStartInfo($"https://{settings.Host}/") { UseShellExecute = true });
    }

    // ---- Settings ----

    private void ApplyHotKeySetting()
    {
        hotKeyWarning = null;
        if (!settings.GlobalHotKeysEnabled)
        {
            globalHotKeys?.UnregisterAll();
            return;
        }

        globalHotKeys ??= new GlobalHotKeys(portIndex => _ = SwitchToPortAsync(portIndex, fromHotKey: true));
        IReadOnlyList<int> unavailablePortNumbers = globalHotKeys.Register(switchState?.Ports.Count ?? DefaultPortCount);
        if (unavailablePortNumbers.Count > 0)
        {
            hotKeyWarning = $"Hotkey {GlobalHotKeys.ModifierDescription}+{string.Join(", ", unavailablePortNumbers)} "
                + "in use by another app";
        }
    }

    private void SettingsDidSave()
    {
        kvmClient.ForgetSession();
        ApplyHotKeySetting();
        UpdateTrayIcon();
        _ = RefreshStateAsync();
    }

    private async Task<string> TestConnectionAsync()
    {
        kvmClient.ForgetSession();
        // Let any background refresh finish so the result reflects the settings just saved.
        while (refreshInProgress)
        {
            await Task.Delay(100);
        }

        await RefreshStateAsync();
        if (lastErrorMessage is not null)
        {
            return $"Failed: {lastErrorMessage}";
        }

        return $"Connected. {switchState?.Ports.Count ?? 0} ports, active: {ActivePortName() ?? "?"}.";
    }

    private void ShowSettings()
    {
        if (settingsForm is null || settingsForm.IsDisposed)
        {
            settingsForm = new SettingsForm(settings, SettingsDidSave, TestConnectionAsync);
        }

        settingsForm.Show();
        settingsForm.WindowState = FormWindowState.Normal;
        settingsForm.Activate();
    }
}
