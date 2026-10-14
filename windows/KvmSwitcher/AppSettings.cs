using System.Text.Json;
using System.Text.Json.Serialization;

namespace KvmSwitcher;

/// <summary>What the tray icon shows for the active port.</summary>
internal enum TrayIconStyle
{
    PortNumber,
    PlainIcon,
}

/// <summary>
/// Non-secret preferences, stored as JSON in %APPDATA%\KVMSwitcher\settings.json.
/// The password lives in Windows Credential Manager (see <see cref="CredentialStore"/>).
/// </summary>
internal sealed class AppSettings
{
    private static readonly JsonSerializerOptions SerializerOptions = new()
    {
        WriteIndented = true,
        Converters = { new JsonStringEnumConverter() },
    };

    private static readonly object SaveLock = new();

    /// <summary>Bare host name or IP of the KVM, e.g. "glkvm.local".</summary>
    public string Host { get; set; } = "";

    /// <summary>Local fallback names, used when the KVM has no name for a port.</summary>
    public List<string> LocalPortNames { get; set; } = new();

    public bool GlobalHotKeysEnabled { get; set; } = true;

    /// <summary>Show a Windows notification after a hotkey switch, since the tray has no visible text.</summary>
    public bool ShowSwitchNotifications { get; set; } = true;

    public TrayIconStyle TrayIconStyle { get; set; } = TrayIconStyle.PortNumber;

    /// <summary>SHA-256 of the KVM's self-signed certificate, pinned on first connection.</summary>
    public string? PinnedCertificateFingerprint { get; set; }

    private static string SettingsDirectory =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "KVMSwitcher");

    private static string SettingsFilePath => Path.Combine(SettingsDirectory, "settings.json");

    public static AppSettings Load()
    {
        try
        {
            if (File.Exists(SettingsFilePath))
            {
                string settingsJson = File.ReadAllText(SettingsFilePath);
                return JsonSerializer.Deserialize<AppSettings>(settingsJson, SerializerOptions) ?? new AppSettings();
            }
        }
        catch (Exception loadError) when (loadError is IOException or JsonException or UnauthorizedAccessException)
        {
            // A damaged settings file should not stop the app; fall back to defaults.
        }

        return new AppSettings();
    }

    public void Save()
    {
        lock (SaveLock)
        {
            Directory.CreateDirectory(SettingsDirectory);
            File.WriteAllText(SettingsFilePath, JsonSerializer.Serialize(this, SerializerOptions));
        }
    }

    public string LocalName(int portIndex)
    {
        return portIndex < LocalPortNames.Count ? LocalPortNames[portIndex].Trim() : "";
    }

    /// <summary>Accepts "https://1.2.3.4/", "1.2.3.4" etc. and returns just the host part.</summary>
    public static string NormalizeHost(string rawHost)
    {
        string trimmedHost = rawHost.Trim();
        foreach (string scheme in new[] { "https://", "http://" })
        {
            if (trimmedHost.StartsWith(scheme, StringComparison.OrdinalIgnoreCase))
            {
                trimmedHost = trimmedHost[scheme.Length..];
            }
        }

        return trimmedHost.TrimEnd('/');
    }
}
