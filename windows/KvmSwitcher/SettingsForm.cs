namespace KvmSwitcher;

/// <summary>Settings window, built in code to keep the project free of designer files.</summary>
internal sealed class SettingsForm : Form
{
    public const int EditablePortCount = 4;

    private readonly AppSettings settings;
    private readonly Action onSave;
    private readonly Func<Task<string>> onTestConnection;

    private readonly TextBox hostTextBox = new() { PlaceholderText = "glkvm.local or IP address", Width = 260 };
    private readonly TextBox passwordTextBox = new() { UseSystemPasswordChar = true, Width = 260 };
    private readonly Button testConnectionButton = new() { Text = "Test Connection", AutoSize = true };
    private readonly Button resetCertificateButton = new() { Text = "Reset Pinned Certificate", AutoSize = true };
    private readonly Label statusLabel = new() { AutoSize = true, MaximumSize = new Size(420, 0), ForeColor = SystemColors.GrayText };
    private readonly TextBox[] portNameTextBoxes = new TextBox[EditablePortCount];
    private readonly ComboBox trayIconStyleComboBox = new() { DropDownStyle = ComboBoxStyle.DropDownList, Width = 260 };
    private readonly CheckBox globalHotKeysCheckBox = new()
    {
        Text = $"Global hotkeys ({GlobalHotKeys.ModifierDescription}+1 – 4)",
        AutoSize = true,
    };

    private readonly CheckBox switchNotificationsCheckBox = new()
    {
        Text = "Show a notification when a hotkey switches ports",
        AutoSize = true,
    };

    private readonly CheckBox launchAtLoginCheckBox = new() { Text = "Launch at login", AutoSize = true };
    private readonly Button saveButton = new() { Text = "Save", AutoSize = true };
    private readonly Button closeButton = new() { Text = "Close", AutoSize = true, DialogResult = DialogResult.Cancel };

    public SettingsForm(AppSettings settings, Action onSave, Func<Task<string>> onTestConnection)
    {
        this.settings = settings;
        this.onSave = onSave;
        this.onTestConnection = onTestConnection;

        Text = "KVM Switcher Settings";
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;
        MinimizeBox = false;
        StartPosition = FormStartPosition.CenterScreen;
        AutoScaleMode = AutoScaleMode.Dpi;
        AutoSize = true;
        AutoSizeMode = AutoSizeMode.GrowAndShrink;
        Padding = new Padding(12);
        Font = SystemFonts.MessageBoxFont ?? Font;
        AcceptButton = saveButton;
        CancelButton = closeButton;

        BuildLayout();
        LoadValues();

        testConnectionButton.Click += async (_, _) => await TestConnectionAsync();
        resetCertificateButton.Click += (_, _) =>
        {
            settings.PinnedCertificateFingerprint = null;
            settings.Save();
            statusLabel.Text = "Certificate will be re-pinned on next connection.";
        };
        saveButton.Click += (_, _) => SaveValues(showStatus: true);
        closeButton.Click += (_, _) => Close();
    }

    private void BuildLayout()
    {
        var mainLayout = new TableLayoutPanel { ColumnCount = 1, AutoSize = true, Dock = DockStyle.Fill };

        var connectionLayout = NewTwoColumnLayout();
        AddRow(connectionLayout, "Address", hostTextBox);
        AddRow(connectionLayout, "Admin password", passwordTextBox);
        var connectionButtons = new FlowLayoutPanel { AutoSize = true, WrapContents = false, Margin = Padding.Empty };
        connectionButtons.Controls.AddRange(new Control[] { testConnectionButton, resetCertificateButton });
        AddRow(connectionLayout, "", connectionButtons);
        AddRow(connectionLayout, "", statusLabel);
        mainLayout.Controls.Add(NewGroup("GL.iNet KVM", connectionLayout));

        var portNamesLayout = NewTwoColumnLayout();
        for (int portIndex = 0; portIndex < EditablePortCount; portIndex++)
        {
            portNameTextBoxes[portIndex] = new TextBox { PlaceholderText = $"Port {portIndex + 1}", Width = 260 };
            AddRow(portNamesLayout, $"Port {portIndex + 1}", portNameTextBoxes[portIndex]);
        }

        mainLayout.Controls.Add(NewGroup("Port names (used when the KVM has none set)", portNamesLayout));

        var generalLayout = NewTwoColumnLayout();
        trayIconStyleComboBox.Items.AddRange(new object[] { "Port number on icon", "Plain icon" });
        AddRow(generalLayout, "Tray icon shows", trayIconStyleComboBox);
        AddRow(generalLayout, "", globalHotKeysCheckBox);
        AddRow(generalLayout, "", switchNotificationsCheckBox);
        AddRow(generalLayout, "", launchAtLoginCheckBox);
        mainLayout.Controls.Add(NewGroup("General", generalLayout));

        var dialogButtons = new FlowLayoutPanel
        {
            FlowDirection = FlowDirection.RightToLeft,
            AutoSize = true,
            Dock = DockStyle.Fill,
        };
        dialogButtons.Controls.AddRange(new Control[] { closeButton, saveButton });
        mainLayout.Controls.Add(dialogButtons);

        Controls.Add(mainLayout);
    }

    private static TableLayoutPanel NewTwoColumnLayout()
    {
        var twoColumnLayout = new TableLayoutPanel { ColumnCount = 2, AutoSize = true, Dock = DockStyle.Fill };
        twoColumnLayout.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 130));
        twoColumnLayout.ColumnStyles.Add(new ColumnStyle(SizeType.AutoSize));
        return twoColumnLayout;
    }

    private static GroupBox NewGroup(string title, Control content)
    {
        var groupBox = new GroupBox
        {
            Text = title,
            AutoSize = true,
            Dock = DockStyle.Fill,
            Padding = new Padding(8),
        };
        groupBox.Controls.Add(content);
        return groupBox;
    }

    private static void AddRow(TableLayoutPanel twoColumnLayout, string labelText, Control rowControl)
    {
        int rowIndex = twoColumnLayout.RowCount++;
        twoColumnLayout.RowStyles.Add(new RowStyle(SizeType.AutoSize));
        var rowLabel = new Label { Text = labelText, AutoSize = true, Anchor = AnchorStyles.Left };
        twoColumnLayout.Controls.Add(rowLabel, 0, rowIndex);
        twoColumnLayout.Controls.Add(rowControl, 1, rowIndex);
    }

    private void LoadValues()
    {
        hostTextBox.Text = settings.Host;
        passwordTextBox.Text = CredentialStore.ReadPassword() ?? "";
        for (int portIndex = 0; portIndex < EditablePortCount; portIndex++)
        {
            portNameTextBoxes[portIndex].Text = settings.LocalName(portIndex);
        }

        trayIconStyleComboBox.SelectedIndex = settings.TrayIconStyle == TrayIconStyle.PlainIcon ? 1 : 0;
        globalHotKeysCheckBox.Checked = settings.GlobalHotKeysEnabled;
        switchNotificationsCheckBox.Checked = settings.ShowSwitchNotifications;
        launchAtLoginCheckBox.Checked = StartupRegistration.IsEnabled;
    }

    private bool SaveValues(bool showStatus)
    {
        try
        {
            settings.Host = AppSettings.NormalizeHost(hostTextBox.Text);
            settings.LocalPortNames = portNameTextBoxes.Select(nameTextBox => nameTextBox.Text.Trim()).ToList();
            settings.TrayIconStyle = trayIconStyleComboBox.SelectedIndex == 1
                ? TrayIconStyle.PlainIcon
                : TrayIconStyle.PortNumber;
            settings.GlobalHotKeysEnabled = globalHotKeysCheckBox.Checked;
            settings.ShowSwitchNotifications = switchNotificationsCheckBox.Checked;
            settings.Save();
            CredentialStore.SavePassword(passwordTextBox.Text);
            StartupRegistration.SetEnabled(launchAtLoginCheckBox.Checked);
        }
        catch (Exception saveError) when (saveError is IOException or UnauthorizedAccessException or InvalidOperationException)
        {
            statusLabel.Text = $"Could not save settings: {saveError.Message}";
            return false;
        }

        hostTextBox.Text = settings.Host;
        onSave();
        if (showStatus)
        {
            statusLabel.Text = "Saved.";
        }

        return true;
    }

    private async Task TestConnectionAsync()
    {
        if (!SaveValues(showStatus: false))
        {
            return;
        }

        testConnectionButton.Enabled = false;
        statusLabel.Text = "Connecting…";
        try
        {
            statusLabel.Text = await onTestConnection();
        }
        finally
        {
            testConnectionButton.Enabled = true;
        }
    }
}
