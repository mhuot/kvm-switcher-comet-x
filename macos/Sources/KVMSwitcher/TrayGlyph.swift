import AppKit

/// The menu bar icon, drawn on the same 16×16 pixel grid as the Windows and Linux apps so it
/// looks the same everywhere. (x, y) is column, row from the top left. Every shape is whole
/// pixels, and the image is a template so macOS tints it for light and dark menu bars.
enum TrayGlyph {
    enum State: Equatable {
        /// The monitor with the port number cut out; numbers outside 1–4 leave it solid.
        case connected(portNumber: Int)
        /// "…" cut out, while switching or before the first reply.
        case busy
        /// "!" cut out over a broken arrow.
        case error
    }

    static let gridSize = 16

    /// 4×5 port digits, cut out of the monitor with their top left at (6, 2).
    private static let digitPatterns: [Int: [String]] = [
        1: [".XX.", "..X.", "..X.", "..X.", ".XXX"],
        2: ["XXX.", "...X", ".XX.", "X...", "XXXX"],
        3: ["XXX.", "...X", ".XX.", "...X", "XXX."],
        4: ["X..X", "X..X", "XXXX", "...X", "...X"],
    ]

    /// The glyph as rows of pixels, true where it is drawn.
    static func pixels(for state: State) -> [[Bool]] {
        var grid = Array(repeating: Array(repeating: false, count: gridSize), count: gridSize)
        func fill(_ left: Int, _ top: Int, _ right: Int, _ bottom: Int, _ value: Bool) {
            for gridY in top ... bottom {
                for gridX in left ... right { grid[gridY][gridX] = value }
            }
        }

        // Monitor, 14×8 (about 16:9) with its corner pixels removed.
        fill(1, 1, 14, 8, true)
        for (cornerX, cornerY) in [(1, 1), (14, 1), (1, 8), (14, 8)] { grid[cornerY][cornerX] = false }
        // Double-headed arrow under the monitor.
        fill(1, 12, 14, 12, true)
        for (headX, headY) in [(2, 11), (2, 13), (3, 10), (3, 14), (13, 11), (13, 13), (12, 10), (12, 14)] {
            grid[headY][headX] = true
        }

        switch state {
        case let .connected(portNumber):
            for (rowIndex, patternRow) in (digitPatterns[portNumber] ?? []).enumerated() {
                for (columnIndex, patternCell) in patternRow.enumerated() where patternCell == "X" {
                    grid[2 + rowIndex][6 + columnIndex] = false
                }
            }
        case .busy:
            fill(3, 4, 4, 5, false)
            fill(7, 4, 8, 5, false)
            fill(11, 4, 12, 5, false)
        case .error:
            fill(7, 2, 8, 4, false)
            fill(7, 6, 8, 6, false)
            fill(7, 12, 8, 12, false)
        }
        return grid
    }

    /// The glyph as text, "#" for drawn pixels, for tests and debugging.
    static func asciiArt(for state: State) -> String {
        pixels(for: state).map { row in String(row.map { $0 ? "#" : "." }) + "\n" }.joined()
    }

    /// A 16 pt template image; on Retina screens each grid pixel becomes a sharp 2×2 block.
    static func image(for state: State) -> NSImage {
        let grid = pixels(for: state)
        let glyphImage = NSImage(size: NSSize(width: gridSize, height: gridSize), flipped: true) { _ in
            NSColor.black.setFill()
            for (gridY, gridRow) in grid.enumerated() {
                for (gridX, pixelSet) in gridRow.enumerated() where pixelSet {
                    NSRect(x: gridX, y: gridY, width: 1, height: 1).fill()
                }
            }
            return true
        }
        glyphImage.isTemplate = true
        glyphImage.accessibilityDescription = "KVM Switcher"
        return glyphImage
    }
}
