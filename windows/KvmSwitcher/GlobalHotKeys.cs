using System.Runtime.InteropServices;

namespace KvmSwitcher;

/// <summary>
/// System-wide Ctrl+Alt+Win+1 … Ctrl+Alt+Win+N hotkeys via RegisterHotKey, delivered to a hidden window.
/// </summary>
internal sealed class GlobalHotKeys : NativeWindow, IDisposable
{
    public const string ModifierDescription = "Ctrl+Alt+Win";
    private const int MaximumHotKeyCount = 9;
    private const int WindowMessageHotKey = 0x0312;
    private const uint ModifierAlt = 0x0001;
    private const uint ModifierControl = 0x0002;
    private const uint ModifierWin = 0x0008;
    private const uint ModifierNoRepeat = 0x4000;

    private readonly Action<int> onPortHotKey;
    private readonly List<int> registeredHotKeyIds = new();

    public GlobalHotKeys(Action<int> onPortHotKey)
    {
        this.onPortHotKey = onPortHotKey;
        CreateHandle(new CreateParams());
    }

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool RegisterHotKey(IntPtr windowHandle, int hotKeyId, uint modifiers, uint virtualKey);

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool UnregisterHotKey(IntPtr windowHandle, int hotKeyId);

    /// <summary>Registers hotkeys for the first <paramref name="portCount"/> ports.</summary>
    /// <returns>Port numbers (1-based) whose hotkey is already taken by another app.</returns>
    public IReadOnlyList<int> Register(int portCount)
    {
        UnregisterAll();
        var unavailablePortNumbers = new List<int>();
        uint hotKeyModifiers = ModifierControl | ModifierAlt | ModifierWin | ModifierNoRepeat;
        for (int portIndex = 0; portIndex < Math.Min(portCount, MaximumHotKeyCount); portIndex++)
        {
            uint numberVirtualKey = (uint)Keys.D1 + (uint)portIndex;
            if (RegisterHotKey(Handle, portIndex, hotKeyModifiers, numberVirtualKey))
            {
                registeredHotKeyIds.Add(portIndex);
            }
            else
            {
                unavailablePortNumbers.Add(portIndex + 1);
            }
        }

        return unavailablePortNumbers;
    }

    public void UnregisterAll()
    {
        foreach (int hotKeyId in registeredHotKeyIds)
        {
            UnregisterHotKey(Handle, hotKeyId);
        }

        registeredHotKeyIds.Clear();
    }

    public void Dispose()
    {
        UnregisterAll();
        DestroyHandle();
    }

    protected override void WndProc(ref Message message)
    {
        if (message.Msg == WindowMessageHotKey)
        {
            onPortHotKey((int)message.WParam);
        }

        base.WndProc(ref message);
    }
}
