@testable import KVMSwitcher
import XCTest

/// The same pictures are pinned in the Windows and Linux tests, so the three apps stay in step.
final class TrayGlyphTests: XCTestCase {
    private static let arrowRows = """
    ................
    ...#........#...
    ..#..........#..
    .##############.
    ..#..........#..
    ...#........#...
    ................

    """

    private static let brokenArrowRows = """
    ................
    ...#........#...
    ..#..........#..
    .######..######.
    ..#..........#..
    ...#........#...
    ................

    """

    func testPortTwo() {
        XCTAssertEqual(TrayGlyph.asciiArt(for: .connected(portNumber: 2)), """
        ................
        ..############..
        .#####...######.
        .########.#####.
        .######..######.
        .#####.########.
        .#####....#####.
        .##############.
        ..############..

        """ + Self.arrowRows)
    }

    func testPortFour() {
        XCTAssertEqual(TrayGlyph.asciiArt(for: .connected(portNumber: 4)), """
        ................
        ..############..
        .#####.##.#####.
        .#####.##.#####.
        .#####....#####.
        .########.#####.
        .########.#####.
        .##############.
        ..############..

        """ + Self.arrowRows)
    }

    func testPortWithoutDigitLeavesMonitorSolid() {
        let monitorRows = TrayGlyph.asciiArt(for: .connected(portNumber: 5)).split(separator: "\n")[2 ... 7]
        XCTAssertTrue(monitorRows.allSatisfy { $0 == ".##############." })
    }

    func testBusy() {
        XCTAssertEqual(TrayGlyph.asciiArt(for: .busy), """
        ................
        ..############..
        .##############.
        .##############.
        .##..##..##..##.
        .##..##..##..##.
        .##############.
        .##############.
        ..############..

        """ + Self.arrowRows)
    }

    func testError() {
        XCTAssertEqual(TrayGlyph.asciiArt(for: .error), """
        ................
        ..############..
        .######..######.
        .######..######.
        .######..######.
        .##############.
        .######..######.
        .##############.
        ..############..

        """ + Self.brokenArrowRows)
    }

    func testImageIsSixteenPointTemplate() {
        let glyphImage = TrayGlyph.image(for: .busy)
        XCTAssertEqual(glyphImage.size, NSSize(width: 16, height: 16))
        XCTAssertTrue(glyphImage.isTemplate)
    }
}
