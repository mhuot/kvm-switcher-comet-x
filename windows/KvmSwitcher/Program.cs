namespace KvmSwitcher;

internal static class Program
{
    [STAThread]
    private static void Main()
    {
        // Two tray icons fighting over the same hotkeys would be confusing; allow one instance.
        using var singleInstanceMutex = new Mutex(true, @"Local\KVMSwitcher-SingleInstance", out bool isFirstInstance);
        if (!isFirstInstance)
        {
            return;
        }

        ApplicationConfiguration.Initialize();
        Application.Run(new TrayApplicationContext());
    }
}
