namespace KvmSwitcher;

internal static class Program
{
    [STAThread]
    private static int Main(string[] commandLineArguments)
    {
        if (commandLineArguments.Length > 0 && commandLineArguments[0] == "--self-test")
        {
            ApplicationConfiguration.Initialize();
            string logPath = commandLineArguments.Length > 1
                ? commandLineArguments[1]
                : Path.Combine(Path.GetTempPath(), "kvmswitcher-self-test.log");
            return SelfTest.Run(logPath);
        }

        // Two tray icons fighting over the same hotkeys would be confusing; allow one instance.
        using var singleInstanceMutex = new Mutex(true, @"Local\KVMSwitcher-SingleInstance", out bool isFirstInstance);
        if (!isFirstInstance)
        {
            return 0;
        }

        ApplicationConfiguration.Initialize();
        Application.Run(new TrayApplicationContext());
        return 0;
    }
}
