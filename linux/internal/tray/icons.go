package tray

import (
	"math"

	"github.com/mhuot/kvm-switcher-comet-x/linux/internal/statusnotifier"
)

// Theme icon names. Panels use these when the icon theme has them (Adwaita, Breeze, Yaru,
// Papirus and others do); the pixmaps below are the fallback, drawn in code so the binary
// needs no image files.
const (
	IconConnected = "video-display-symbolic"
	IconError     = "dialog-warning-symbolic"
)

var pixmapSizes = []int{16, 22, 24, 32, 48}

// Mid grey reads on both light and dark panels.
const (
	glyphRed   = 0x8a
	glyphGreen = 0x8a
	glyphBlue  = 0x8a
)

// shape reports whether a point in a 24×24 design grid is inside the glyph.
type shape func(gridX, gridY float64) bool

func insideRectangle(gridX, gridY, left, top, right, bottom float64) bool {
	return gridX >= left && gridX <= right && gridY >= top && gridY <= bottom
}

// displayShape is a monitor outline on a stand.
func displayShape(gridX, gridY float64) bool {
	insideOuter := insideRectangle(gridX, gridY, 2, 4, 22, 17)
	insideScreen := insideRectangle(gridX, gridY, 4, 6, 20, 15)
	stand := insideRectangle(gridX, gridY, 11, 17, 13, 19.5)
	base := insideRectangle(gridX, gridY, 7, 19.5, 17, 21)
	return (insideOuter && !insideScreen) || stand || base
}

// warningShape is a triangle with an exclamation mark cut out.
func warningShape(gridX, gridY float64) bool {
	// Triangle with apex (12,3) and base from (1.5,21) to (22.5,21).
	if gridY < 3 || gridY > 21 {
		return false
	}
	halfWidth := (gridY - 3) / 18 * 10.5
	if math.Abs(gridX-12) > halfWidth {
		return false
	}
	exclamationBar := insideRectangle(gridX, gridY, 11, 9, 13, 15)
	exclamationDot := math.Hypot(gridX-12, gridY-17.5) <= 1.3
	return !exclamationBar && !exclamationDot
}

// renderPixmaps draws a glyph at every panel size, with 4×4 supersampling for smooth edges.
func renderPixmaps(glyph shape) []statusnotifier.Pixmap {
	const samplesPerAxis = 4
	pixmaps := make([]statusnotifier.Pixmap, 0, len(pixmapSizes))
	for _, size := range pixmapSizes {
		pixelData := make([]byte, 0, size*size*4)
		gridUnitsPerPixel := 24 / float64(size)
		for pixelY := range size {
			for pixelX := range size {
				coveredSamples := 0
				for sampleY := range samplesPerAxis {
					for sampleX := range samplesPerAxis {
						gridX := (float64(pixelX) + (float64(sampleX)+0.5)/samplesPerAxis) * gridUnitsPerPixel
						gridY := (float64(pixelY) + (float64(sampleY)+0.5)/samplesPerAxis) * gridUnitsPerPixel
						if glyph(gridX, gridY) {
							coveredSamples++
						}
					}
				}
				alpha := byte(coveredSamples * 255 / (samplesPerAxis * samplesPerAxis))
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

var (
	connectedPixmaps = renderPixmaps(displayShape)
	errorPixmaps     = renderPixmaps(warningShape)
)
