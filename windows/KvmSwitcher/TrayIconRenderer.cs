using System.Drawing.Imaging;
using System.Runtime.InteropServices;
using Microsoft.Win32;

namespace KvmSwitcher;

/// <summary>
/// Draws the tray icon at runtime from <see cref="TrayGlyph"/>. The Windows tray cannot show text
/// beside an icon the way the macOS menu bar can, so the active port number is part of the icon.
/// </summary>
internal static class TrayIconRenderer
{
    private const string PersonalizeKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize";

    [DllImport("user32.dll")]
    private static extern bool DestroyIcon(IntPtr iconHandle);

    /// <param name="portNumber">Port cut out of the connected monitor; 0 for a plain monitor.</param>
    public static Icon Render(TrayGlyph.GlyphState glyphState, int portNumber)
    {
        bool[,] glyphPixels = TrayGlyph.Pixels(glyphState, portNumber);

        // Draw at the tray's own size using the largest whole-number scale of the 16 px grid that
        // fits, centred, so Windows never resamples it and the edges stay sharp.
        int iconPixelSize = SystemInformation.SmallIconSize.Width;
        int glyphScale = Math.Max(1, iconPixelSize / TrayGlyph.GridSize);
        int glyphOffset = (iconPixelSize - TrayGlyph.GridSize * glyphScale) / 2;

        using var iconBitmap = new Bitmap(iconPixelSize, iconPixelSize, PixelFormat.Format32bppArgb);
        using (Graphics iconGraphics = Graphics.FromImage(iconBitmap))
        using (var glyphBrush = new SolidBrush(TaskbarUsesLightTheme() ? Color.Black : Color.White))
        {
            iconGraphics.Clear(Color.Transparent);
            for (int gridY = 0; gridY < TrayGlyph.GridSize; gridY++)
            {
                for (int gridX = 0; gridX < TrayGlyph.GridSize; gridX++)
                {
                    if (glyphPixels[gridY, gridX])
                    {
                        iconGraphics.FillRectangle(glyphBrush,
                            glyphOffset + gridX * glyphScale, glyphOffset + gridY * glyphScale, glyphScale, glyphScale);
                    }
                }
            }
        }

        IntPtr iconHandle = iconBitmap.GetHicon();
        try
        {
            using Icon borrowedIcon = Icon.FromHandle(iconHandle);
            return (Icon)borrowedIcon.Clone();
        }
        finally
        {
            DestroyIcon(iconHandle);
        }
    }

    /// <summary>The taskbar follows the "Windows mode" setting, separate from the app mode.</summary>
    private static bool TaskbarUsesLightTheme()
    {
        using RegistryKey? personalizeKey = Registry.CurrentUser.OpenSubKey(PersonalizeKeyPath);
        return personalizeKey?.GetValue("SystemUsesLightTheme") is int lightThemeFlag && lightThemeFlag == 1;
    }
}
