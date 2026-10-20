using KvmSwitcher;
using Xunit;

namespace KvmSwitcher.Tests;

/// <summary>The same pictures are pinned in the macOS and Linux tests, so the three apps stay in step.</summary>
public sealed class TrayGlyphTests
{
    private const string ArrowRows =
        "................\n" +
        "...#........#...\n" +
        "..#..........#..\n" +
        ".##############.\n" +
        "..#..........#..\n" +
        "...#........#...\n" +
        "................\n";

    private const string BrokenArrowRows =
        "................\n" +
        "...#........#...\n" +
        "..#..........#..\n" +
        ".######..######.\n" +
        "..#..........#..\n" +
        "...#........#...\n" +
        "................\n";

    [Fact]
    public void PortTwo()
    {
        Assert.Equal(
            "................\n" +
            "..############..\n" +
            ".#####...######.\n" +
            ".########.#####.\n" +
            ".######..######.\n" +
            ".#####.########.\n" +
            ".#####....#####.\n" +
            ".##############.\n" +
            "..############..\n" + ArrowRows,
            TrayGlyph.AsciiArt(TrayGlyph.GlyphState.Connected, 2));
    }

    [Fact]
    public void PortThree()
    {
        Assert.Equal(
            "................\n" +
            "..############..\n" +
            ".#####...######.\n" +
            ".########.#####.\n" +
            ".######..######.\n" +
            ".########.#####.\n" +
            ".#####...######.\n" +
            ".##############.\n" +
            "..############..\n" + ArrowRows,
            TrayGlyph.AsciiArt(TrayGlyph.GlyphState.Connected, 3));
    }

    [Fact]
    public void PlainIconLeavesMonitorSolid()
    {
        string[] glyphRows = TrayGlyph.AsciiArt(TrayGlyph.GlyphState.Connected, 0).Split('\n');
        Assert.All(glyphRows[2..8], glyphRow => Assert.Equal(".##############.", glyphRow));
    }

    [Fact]
    public void Busy()
    {
        Assert.Equal(
            "................\n" +
            "..############..\n" +
            ".##############.\n" +
            ".##############.\n" +
            ".##..##..##..##.\n" +
            ".##..##..##..##.\n" +
            ".##############.\n" +
            ".##############.\n" +
            "..############..\n" + ArrowRows,
            TrayGlyph.AsciiArt(TrayGlyph.GlyphState.Busy, 0));
    }

    [Fact]
    public void Error()
    {
        Assert.Equal(
            "................\n" +
            "..############..\n" +
            ".######..######.\n" +
            ".######..######.\n" +
            ".######..######.\n" +
            ".##############.\n" +
            ".######..######.\n" +
            ".##############.\n" +
            "..############..\n" + BrokenArrowRows,
            TrayGlyph.AsciiArt(TrayGlyph.GlyphState.Error, 0));
    }
}
