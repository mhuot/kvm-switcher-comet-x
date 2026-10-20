package tray

import (
	"strings"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/statusnotifier"
)

// The panel glyph is drawn on a 16×16 pixel grid, the same grid the macOS and Windows apps
// use, so the icon looks the same everywhere. (x, y) is column, row from the top left. Every
// shape is whole pixels, so integer scales stay sharp. The pixmaps are drawn in code so the
// binary needs no image files.

// GlyphState is which of the three icon designs to draw.
type GlyphState int

const (
	// GlyphConnected is the monitor with the active port number cut out of it.
	GlyphConnected GlyphState = iota
	// GlyphBusy is the monitor with "…" cut out, while switching or before the first reply.
	GlyphBusy
	// GlyphError is the monitor with "!" cut out over a broken arrow.
	GlyphError
)

const glyphGridSize = 16

var pixmapSizes = []int{16, 22, 24, 32, 48}

// Mid grey reads on both light and dark panels.
const (
	glyphRed   = 0x8a
	glyphGreen = 0x8a
	glyphBlue  = 0x8a
)

// digitPatterns are 4×5 port digits, cut out of the monitor with their top left at (6, 2).
var digitPatterns = map[int][]string{
	1: {".XX.", "..X.", "..X.", "..X.", ".XXX"},
	2: {"XXX.", "...X", ".XX.", "X...", "XXXX"},
	3: {"XXX.", "...X", ".XX.", "...X", "XXX."},
	4: {"X..X", "X..X", "XXXX", "...X", "...X"},
}

type glyphGrid [glyphGridSize][glyphGridSize]bool

// fill sets every pixel from (left, top) to (right, bottom), inclusive.
func (grid *glyphGrid) fill(left, top, right, bottom int, value bool) {
	for gridY := top; gridY <= bottom; gridY++ {
		for gridX := left; gridX <= right; gridX++ {
			grid[gridY][gridX] = value
		}
	}
}

// renderGlyph draws a state. portNumber 1-4 is cut out of the connected monitor; any other
// value leaves the monitor solid.
func renderGlyph(state GlyphState, portNumber int) glyphGrid {
	var grid glyphGrid
	// Monitor, 14×8 (about 16:9) with its corner pixels removed.
	grid.fill(1, 1, 14, 8, true)
	for _, corner := range [][2]int{{1, 1}, {14, 1}, {1, 8}, {14, 8}} {
		grid[corner[1]][corner[0]] = false
	}
	// Double-headed arrow under the monitor.
	grid.fill(1, 12, 14, 12, true)
	for _, arrowhead := range [][2]int{{2, 11}, {2, 13}, {3, 10}, {3, 14}, {13, 11}, {13, 13}, {12, 10}, {12, 14}} {
		grid[arrowhead[1]][arrowhead[0]] = true
	}

	switch state {
	case GlyphConnected:
		for rowIndex, patternRow := range digitPatterns[portNumber] {
			for columnIndex, patternCell := range patternRow {
				if patternCell == 'X' {
					grid[2+rowIndex][6+columnIndex] = false
				}
			}
		}
	case GlyphBusy:
		grid.fill(3, 4, 4, 5, false)
		grid.fill(7, 4, 8, 5, false)
		grid.fill(11, 4, 12, 5, false)
	case GlyphError:
		grid.fill(7, 2, 8, 4, false)
		grid.fill(7, 6, 8, 6, false)
		grid.fill(7, 12, 8, 12, false)
	}
	return grid
}

// String draws the grid as text, "#" for set pixels, for tests and debugging.
func (grid glyphGrid) String() string {
	var builder strings.Builder
	for _, gridRow := range grid {
		for _, pixelSet := range gridRow {
			if pixelSet {
				builder.WriteByte('#')
			} else {
				builder.WriteByte('.')
			}
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

// GlyphPixmaps draws a state at every panel size. Each size uses the largest whole-number
// scale of the 16 px grid that fits, centred, so edges never blur.
func GlyphPixmaps(state GlyphState, portNumber int) []statusnotifier.Pixmap {
	grid := renderGlyph(state, portNumber)
	pixmaps := make([]statusnotifier.Pixmap, 0, len(pixmapSizes))
	for _, size := range pixmapSizes {
		scale := size / glyphGridSize
		offset := (size - glyphGridSize*scale) / 2
		pixelData := make([]byte, 0, size*size*4)
		for pixelY := range size {
			for pixelX := range size {
				gridX := (pixelX - offset) / scale
				gridY := (pixelY - offset) / scale
				insideGrid := pixelX >= offset && pixelY >= offset && gridX < glyphGridSize && gridY < glyphGridSize
				var alpha byte
				if insideGrid && grid[gridY][gridX] {
					alpha = 0xff
				}
				// ARGB32 in network byte order.
				pixelData = append(pixelData, alpha, glyphRed, glyphGreen, glyphBlue)
			}
		}
		pixmaps = append(pixmaps, statusnotifier.Pixmap{
			Width: int32(size), Height: int32(size), Data: pixelData,
		})
	}
	return pixmaps
}
