package tray

import (
	"strings"
	"testing"
)

// The same pictures are pinned in the macOS and Windows tests, so the three apps stay in step.
const (
	arrowRows = `
................
...#........#...
..#..........#..
.##############.
..#..........#..
...#........#...
................
`
	brokenArrowRows = `
................
...#........#...
..#..........#..
.######..######.
..#..........#..
...#........#...
................
`
)

func expectedGlyph(monitorRows, arrow string) string {
	return strings.TrimPrefix(monitorRows, "\n") + strings.TrimPrefix(arrow, "\n")
}

func TestRenderGlyph(t *testing.T) {
	testCases := []struct {
		name       string
		state      GlyphState
		portNumber int
		want       string
	}{
		{"port 1", GlyphConnected, 1, expectedGlyph(`
................
..############..
.######..######.
.#######.######.
.#######.######.
.#######.######.
.######...#####.
.##############.
..############..
`, arrowRows)},
		{"port 2", GlyphConnected, 2, expectedGlyph(`
................
..############..
.#####...######.
.########.#####.
.######..######.
.#####.########.
.#####....#####.
.##############.
..############..
`, arrowRows)},
		{"port 3", GlyphConnected, 3, expectedGlyph(`
................
..############..
.#####...######.
.########.#####.
.######..######.
.########.#####.
.#####...######.
.##############.
..############..
`, arrowRows)},
		{"port 4", GlyphConnected, 4, expectedGlyph(`
................
..############..
.#####.##.#####.
.#####.##.#####.
.#####....#####.
.########.#####.
.########.#####.
.##############.
..############..
`, arrowRows)},
		{"port without a digit", GlyphConnected, 5, expectedGlyph(`
................
..############..
.##############.
.##############.
.##############.
.##############.
.##############.
.##############.
..############..
`, arrowRows)},
		{"busy", GlyphBusy, 0, expectedGlyph(`
................
..############..
.##############.
.##############.
.##..##..##..##.
.##..##..##..##.
.##############.
.##############.
..############..
`, arrowRows)},
		{"error", GlyphError, 0, expectedGlyph(`
................
..############..
.######..######.
.######..######.
.######..######.
.##############.
.######..######.
.##############.
..############..
`, brokenArrowRows)},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := renderGlyph(testCase.state, testCase.portNumber).String(); got != testCase.want {
				t.Errorf("got\n%s\nwant\n%s", got, testCase.want)
			}
		})
	}
}

func TestGlyphPixmapsScaleWithoutBlur(t *testing.T) {
	setGridPixels := strings.Count(renderGlyph(GlyphConnected, 2).String(), "#")
	pixmaps := GlyphPixmaps(GlyphConnected, 2)
	if len(pixmaps) != len(pixmapSizes) {
		t.Fatalf("%d pixmaps, want %d", len(pixmaps), len(pixmapSizes))
	}
	for _, pixmap := range pixmaps {
		size := int(pixmap.Width)
		if len(pixmap.Data) != size*size*4 {
			t.Fatalf("%d px pixmap has %d bytes", size, len(pixmap.Data))
		}
		setPixels := 0
		for pixelIndex := range size * size {
			switch alpha := pixmap.Data[pixelIndex*4]; alpha {
			case 0xff:
				setPixels++
			case 0x00:
			default:
				t.Fatalf("%d px pixmap has partial alpha %#x", size, alpha)
			}
		}
		scale := size / glyphGridSize
		if wantPixels := setGridPixels * scale * scale; setPixels != wantPixels {
			t.Errorf("%d px pixmap has %d set pixels, want %d", size, setPixels, wantPixels)
		}
	}
}
