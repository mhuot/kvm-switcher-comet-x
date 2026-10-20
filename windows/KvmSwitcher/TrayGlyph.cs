using System.Text;

namespace KvmSwitcher;

/// <summary>
/// The tray icon's picture, on the same 16×16 pixel grid as the macOS and Linux apps so it looks
/// the same everywhere. (x, y) is column, row from the top left; every shape is whole pixels, so
/// whole-number scales stay sharp. Kept free of Windows APIs so the tests run on any OS.
/// </summary>
internal static class TrayGlyph
{
    public enum GlyphState
    {
        /// <summary>The monitor with the port number cut out; numbers outside 1–4 leave it solid.</summary>
        Connected,

        /// <summary>"…" cut out, while switching or before the first reply.</summary>
        Busy,

        /// <summary>"!" cut out over a broken arrow.</summary>
        Error,
    }

    public const int GridSize = 16;

    /// <summary>4×5 port digits, cut out of the monitor with their top left at (6, 2).</summary>
    private static readonly Dictionary<int, string[]> DigitPatterns = new()
    {
        [1] = new[] { ".XX.", "..X.", "..X.", "..X.", ".XXX" },
        [2] = new[] { "XXX.", "...X", ".XX.", "X...", "XXXX" },
        [3] = new[] { "XXX.", "...X", ".XX.", "...X", "XXX." },
        [4] = new[] { "X..X", "X..X", "XXXX", "...X", "...X" },
    };

    /// <summary>The glyph as [row, column] pixels, true where it is drawn.</summary>
    public static bool[,] Pixels(GlyphState glyphState, int portNumber)
    {
        var grid = new bool[GridSize, GridSize];

        void Fill(int left, int top, int right, int bottom, bool value)
        {
            for (int gridY = top; gridY <= bottom; gridY++)
            {
                for (int gridX = left; gridX <= right; gridX++)
                {
                    grid[gridY, gridX] = value;
                }
            }
        }

        // Monitor, 14×8 (about 16:9) with its corner pixels removed.
        Fill(1, 1, 14, 8, true);
        foreach ((int cornerX, int cornerY) in new[] { (1, 1), (14, 1), (1, 8), (14, 8) })
        {
            grid[cornerY, cornerX] = false;
        }

        // Double-headed arrow under the monitor.
        Fill(1, 12, 14, 12, true);
        foreach ((int headX, int headY) in new[] { (2, 11), (2, 13), (3, 10), (3, 14), (13, 11), (13, 13), (12, 10), (12, 14) })
        {
            grid[headY, headX] = true;
        }

        switch (glyphState)
        {
            case GlyphState.Connected when DigitPatterns.TryGetValue(portNumber, out string[]? digitRows):
                for (int rowIndex = 0; rowIndex < digitRows.Length; rowIndex++)
                {
                    for (int columnIndex = 0; columnIndex < digitRows[rowIndex].Length; columnIndex++)
                    {
                        if (digitRows[rowIndex][columnIndex] == 'X')
                        {
                            grid[2 + rowIndex, 6 + columnIndex] = false;
                        }
                    }
                }

                break;
            case GlyphState.Busy:
                Fill(3, 4, 4, 5, false);
                Fill(7, 4, 8, 5, false);
                Fill(11, 4, 12, 5, false);
                break;
            case GlyphState.Error:
                Fill(7, 2, 8, 4, false);
                Fill(7, 6, 8, 6, false);
                Fill(7, 12, 8, 12, false);
                break;
        }

        return grid;
    }

    /// <summary>The glyph as text, "#" for drawn pixels, for tests and debugging.</summary>
    public static string AsciiArt(GlyphState glyphState, int portNumber)
    {
        bool[,] grid = Pixels(glyphState, portNumber);
        var asciiBuilder = new StringBuilder();
        for (int gridY = 0; gridY < GridSize; gridY++)
        {
            for (int gridX = 0; gridX < GridSize; gridX++)
            {
                asciiBuilder.Append(grid[gridY, gridX] ? '#' : '.');
            }

            asciiBuilder.Append('\n');
        }

        return asciiBuilder.ToString();
    }
}
