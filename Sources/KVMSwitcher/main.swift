import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private let statusMenuController = StatusMenuController()

    func applicationDidFinishLaunching(_ notification: Notification) {
        statusMenuController.start()
    }
}

MainActor.assumeIsolated {
    let application = NSApplication.shared
    let appDelegate = AppDelegate()
    application.delegate = appDelegate
    application.setActivationPolicy(.accessory)
    application.run()
}
