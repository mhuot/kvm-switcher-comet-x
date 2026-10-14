using Microsoft.Win32;

namespace KvmSwitcher;

/// <summary>"Launch at login" via the per-user Run key (no admin rights needed).</summary>
internal static class StartupRegistration
{
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string RunValueName = "KVMSwitcher";

    public static bool IsEnabled
    {
        get
        {
            using RegistryKey? runKey = Registry.CurrentUser.OpenSubKey(RunKeyPath);
            return runKey?.GetValue(RunValueName) is string;
        }
    }

    public static void SetEnabled(bool launchAtLogin)
    {
        using RegistryKey runKey = Registry.CurrentUser.CreateSubKey(RunKeyPath);
        if (launchAtLogin && Environment.ProcessPath is { } executablePath)
        {
            runKey.SetValue(RunValueName, $"\"{executablePath}\"");
        }
        else
        {
            runKey.DeleteValue(RunValueName, throwOnMissingValue: false);
        }
    }
}
