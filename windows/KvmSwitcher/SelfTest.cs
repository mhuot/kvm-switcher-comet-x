namespace KvmSwitcher;

/// <summary>
/// `KVMSwitcher.exe --self-test [logPath]` exercises the Windows-only pieces (GDI+ icon drawing,
/// WinForms layout, hotkey window, Credential Manager) without a KVM, then exits.
/// Exit code 0 means every check passed. Used by CI on real Windows runners.
/// </summary>
internal static class SelfTest
{
    private const string SelfTestCredentialTarget = "KVMSwitcher/self-test";

    public static int Run(string logPath)
    {
        var reportLines = new List<string>();
        bool allPassed = true;

        void Check(string checkName, Action checkAction)
        {
            try
            {
                checkAction();
                reportLines.Add($"PASS {checkName}");
            }
            catch (Exception checkError)
            {
                allPassed = false;
                reportLines.Add($"FAIL {checkName}: {checkError.GetType().Name}: {checkError.Message}");
            }
        }

        Check("render tray icons", () =>
        {
            foreach (TrayIconRenderer.IconState iconState in Enum.GetValues<TrayIconRenderer.IconState>())
            {
                foreach (bool plainIcon in new[] { false, true })
                {
                    using Icon renderedIcon = TrayIconRenderer.Render(iconState, "3", plainIcon);
                    if (renderedIcon.Width <= 0)
                    {
                        throw new InvalidOperationException($"Empty icon for {iconState}");
                    }
                }
            }
        });

        Check("build settings window", () =>
        {
            using var settingsForm = new SettingsForm(new AppSettings(), () => { }, () => Task.FromResult(""));
            settingsForm.CreateControl();
            settingsForm.PerformLayout();
            if (settingsForm.PreferredSize.Height < 200)
            {
                throw new InvalidOperationException($"Settings window collapsed to {settingsForm.PreferredSize}");
            }
        });

        Check("create hotkey window", () =>
        {
            using var globalHotKeys = new GlobalHotKeys(_ => { });
            globalHotKeys.Register(SettingsForm.EditablePortCount);
        });

        Check("credential manager round trip", () =>
        {
            const string probePassword = "self-test p&ss ü";
            try
            {
                CredentialStore.SavePassword(probePassword, SelfTestCredentialTarget);
                string? readBack = CredentialStore.ReadPassword(SelfTestCredentialTarget);
                if (readBack != probePassword)
                {
                    throw new InvalidOperationException($"Read back '{readBack}'");
                }
            }
            finally
            {
                CredentialStore.SavePassword(null, SelfTestCredentialTarget);
            }

            if (CredentialStore.ReadPassword(SelfTestCredentialTarget) is not null)
            {
                throw new InvalidOperationException("Credential was not deleted");
            }
        });

        reportLines.Add(allPassed ? "RESULT PASS" : "RESULT FAIL");
        File.WriteAllLines(logPath, reportLines);
        return allPassed ? 0 : 1;
    }
}
