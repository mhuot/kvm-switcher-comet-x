using System.Drawing.Drawing2D;
using System.Drawing.Text;
using System.Runtime.InteropServices;

namespace KvmSwitcher;

/// <summary>
/// Draws the tray icon at runtime. The Windows tray cannot show text beside an icon the way the
/// macOS menu bar can, so the active port number is drawn onto the icon itself.
/// </summary>
internal static class TrayIconRenderer
{
    public enum IconState
    {
        Connected,
        Switching,
        NotConnected,
        Error,
    }

    private const int IconPixelSize = 32;
    private static readonly Color ConnectedColor = Color.FromArgb(0, 103, 192);
    private static readonly Color SwitchingColor = Color.FromArgb(96, 125, 139);
    private static readonly Color NotConnectedColor = Color.FromArgb(117, 117, 117);
    private static readonly Color ErrorColor = Color.FromArgb(196, 43, 28);

    [DllImport("user32.dll")]
    private static extern bool DestroyIcon(IntPtr iconHandle);

    /// <param name="badgeText">Text drawn on the badge, e.g. "3"; ignored for the plain icon.</param>
    public static Icon Render(IconState iconState, string badgeText, bool plainIcon)
    {
        using var iconBitmap = new Bitmap(IconPixelSize, IconPixelSize);
        using (Graphics iconGraphics = Graphics.FromImage(iconBitmap))
        {
            iconGraphics.SmoothingMode = SmoothingMode.AntiAlias;
            iconGraphics.TextRenderingHint = TextRenderingHint.AntiAliasGridFit;
            iconGraphics.Clear(Color.Transparent);

            Color fillColor = iconState switch
            {
                IconState.Connected => ConnectedColor,
                IconState.Switching => SwitchingColor,
                IconState.Error => ErrorColor,
                _ => NotConnectedColor,
            };

            if (plainIcon && iconState == IconState.Connected)
            {
                DrawMonitor(iconGraphics, fillColor);
            }
            else
            {
                string displayedText = iconState switch
                {
                    IconState.Switching => "…",
                    IconState.Error => "!",
                    IconState.NotConnected => "–",
                    _ => badgeText,
                };
                DrawBadge(iconGraphics, fillColor, displayedText);
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

    private static void DrawBadge(Graphics iconGraphics, Color fillColor, string badgeText)
    {
        var badgeBounds = new Rectangle(1, 1, IconPixelSize - 2, IconPixelSize - 2);
        using (GraphicsPath badgePath = RoundedRectangle(badgeBounds, 7))
        using (var badgeBrush = new SolidBrush(fillColor))
        {
            iconGraphics.FillPath(badgeBrush, badgePath);
        }

        float fontSize = badgeText.Length > 1 ? 15f : 20f;
        using var badgeFont = new Font("Segoe UI", fontSize, FontStyle.Bold, GraphicsUnit.Pixel);
        using var centeredFormat = new StringFormat
        {
            Alignment = StringAlignment.Center,
            LineAlignment = StringAlignment.Center,
        };
        iconGraphics.DrawString(badgeText, badgeFont, Brushes.White, badgeBounds, centeredFormat);
    }

    private static void DrawMonitor(Graphics iconGraphics, Color fillColor)
    {
        using var monitorBrush = new SolidBrush(fillColor);
        using (GraphicsPath screenPath = RoundedRectangle(new Rectangle(2, 4, 28, 19), 3))
        {
            iconGraphics.FillPath(monitorBrush, screenPath);
        }

        iconGraphics.FillRectangle(monitorBrush, 13, 23, 6, 4);
        iconGraphics.FillRectangle(monitorBrush, 8, 27, 16, 3);
    }

    private static GraphicsPath RoundedRectangle(Rectangle bounds, int cornerRadius)
    {
        int cornerDiameter = cornerRadius * 2;
        var roundedPath = new GraphicsPath();
        roundedPath.AddArc(bounds.Left, bounds.Top, cornerDiameter, cornerDiameter, 180, 90);
        roundedPath.AddArc(bounds.Right - cornerDiameter, bounds.Top, cornerDiameter, cornerDiameter, 270, 90);
        roundedPath.AddArc(bounds.Right - cornerDiameter, bounds.Bottom - cornerDiameter, cornerDiameter, cornerDiameter, 0, 90);
        roundedPath.AddArc(bounds.Left, bounds.Bottom - cornerDiameter, cornerDiameter, cornerDiameter, 90, 90);
        roundedPath.CloseFigure();
        return roundedPath;
    }
}
