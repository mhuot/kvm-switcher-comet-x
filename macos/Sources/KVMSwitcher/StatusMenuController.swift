import AppKit
import SwiftUI

/// Owns the menu bar item, the port menu and the settings window.
@MainActor
final class StatusMenuController: NSObject, NSMenuDelegate {
    private static let backgroundRefreshInterval: TimeInterval = 30
    private static let defaultPortCount = 4
    private static let maximumMenuBarNameLength = 24

    private let kvmClient = KVMClient()
    private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let portMenu = NSMenu()
    private var globalHotKeys: GlobalHotKeys?
    private var settingsWindow: NSWindow?
    private var refreshTimer: Timer?

    private var switchState: KVMSwitchState?
    private var lastErrorMessage: String?
    private var switchInProgress = false

    func start() {
        portMenu.delegate = self
        portMenu.autoenablesItems = false
        statusItem.menu = portMenu
        updateStatusItemTitle()
        rebuildMenu()
        applyHotKeySetting()

        refreshTimer = Timer.scheduledTimer(withTimeInterval: Self.backgroundRefreshInterval, repeats: true) {
            [weak self] _ in
            Task { @MainActor in await self?.refreshState() }
        }
        Task { await refreshState() }

        if AppSettings.host.isEmpty || KeychainStore.password == nil {
            showSettings()
        }
    }

    // MARK: - State

    private func refreshState() async {
        do {
            switchState = try await kvmClient.fetchSwitchState()
            lastErrorMessage = nil
        } catch {
            lastErrorMessage = error.localizedDescription
        }
        updateStatusItemTitle()
        rebuildMenu()
    }

    private func switchToPort(_ portIndex: Int) {
        if let switchState, portIndex >= switchState.ports.count { return }
        guard !switchInProgress else { return }
        switchInProgress = true
        updateStatusItemTitle()
        Task {
            defer { switchInProgress = false }
            do {
                try await kvmClient.setActivePort(portIndex)
                lastErrorMessage = nil
                // The switch reports "synced: false" briefly; give it a moment before reading back.
                try? await Task.sleep(nanoseconds: 400_000_000)
            } catch {
                lastErrorMessage = error.localizedDescription
                NSSound.beep()
            }
            await refreshState()
        }
    }

    // MARK: - Menu bar

    private func updateStatusItemTitle() {
        guard let button = statusItem.button else { return }
        button.imagePosition = .imageLeading
        if lastErrorMessage != nil {
            button.image = TrayGlyph.image(for: .error)
            button.title = ""
        } else if let switchState, !switchInProgress {
            button.image = TrayGlyph.image(for: .connected(portNumber: switchState.activePortIndex + 1))
            button.title = menuBarLabel()
        } else {
            button.image = TrayGlyph.image(for: .busy)
            button.title = ""
        }
        button.toolTip = lastErrorMessage ?? activePortName().map { "KVM: \($0)" }
    }

    private func menuBarLabel() -> String {
        switch AppSettings.menuBarLabelStyle {
        case .iconOnly:
            return ""
        case .portName:
            guard let portName = activePortName() else { return "" }
            // Keep long names from crowding out other menu bar items.
            guard portName.count > Self.maximumMenuBarNameLength else { return " \(portName)" }
            return " \(portName.prefix(Self.maximumMenuBarNameLength - 1))…"
        }
    }

    private func activePortName() -> String? {
        guard let switchState, switchState.activePortIndex < switchState.ports.count else { return nil }
        return switchState.ports[switchState.activePortIndex].displayName
    }

    private func rebuildMenu() {
        portMenu.removeAllItems()

        if let lastErrorMessage {
            let errorItem = NSMenuItem(title: lastErrorMessage, action: nil, keyEquivalent: "")
            errorItem.isEnabled = false
            portMenu.addItem(errorItem)
            portMenu.addItem(.separator())
        }

        let ports = switchState?.ports ?? (0 ..< Self.defaultPortCount).map {
            KVMPort(portIndex: $0, portId: "", deviceName: "", hasVideoLink: true)
        }
        for port in ports {
            var title = port.displayName
            if !port.hasVideoLink { title += "  (no video)" }
            let portItem = NSMenuItem(
                title: title,
                action: #selector(portMenuItemSelected(_:)),
                keyEquivalent: port.portIndex < 9 ? String(port.portIndex + 1) : ""
            )
            portItem.keyEquivalentModifierMask = []
            portItem.target = self
            portItem.tag = port.portIndex
            portItem.state = port.portIndex == switchState?.activePortIndex ? .on : .off
            portItem.isEnabled = switchState != nil
            portMenu.addItem(portItem)
        }

        portMenu.addItem(.separator())
        if AppSettings.globalHotKeysEnabled {
            let hintItem = NSMenuItem(
                title: "Hotkeys: \(GlobalHotKeys.modifierDescription)1 – \(ports.count)",
                action: nil, keyEquivalent: ""
            )
            hintItem.isEnabled = false
            portMenu.addItem(hintItem)
        }
        addActionItem("Refresh", #selector(refreshMenuItemSelected), keyEquivalent: "r")
        addActionItem("Open KVM Web UI", #selector(openWebUIMenuItemSelected), keyEquivalent: "o")
        addActionItem("Settings…", #selector(settingsMenuItemSelected), keyEquivalent: ",")
        portMenu.addItem(.separator())
        addActionItem("Quit KVM Switcher", #selector(quitMenuItemSelected), keyEquivalent: "q")
    }

    private func addActionItem(_ title: String, _ action: Selector, keyEquivalent: String) {
        let actionItem = NSMenuItem(title: title, action: action, keyEquivalent: keyEquivalent)
        actionItem.target = self
        portMenu.addItem(actionItem)
    }

    nonisolated func menuWillOpen(_ menu: NSMenu) {
        Task { @MainActor in await self.refreshState() }
    }

    // MARK: - Actions

    @objc private func portMenuItemSelected(_ sender: NSMenuItem) {
        switchToPort(sender.tag)
    }

    @objc private func refreshMenuItemSelected() {
        Task { await refreshState() }
    }

    @objc private func openWebUIMenuItemSelected() {
        guard !AppSettings.host.isEmpty, let webUIURL = URL(string: "https://\(AppSettings.host)/") else { return }
        NSWorkspace.shared.open(webUIURL)
    }

    @objc private func settingsMenuItemSelected() {
        showSettings()
    }

    @objc private func quitMenuItemSelected() {
        NSApp.terminate(nil)
    }

    // MARK: - Settings

    private func applyHotKeySetting() {
        if AppSettings.globalHotKeysEnabled {
            if globalHotKeys == nil {
                globalHotKeys = GlobalHotKeys { [weak self] portIndex in
                    Task { @MainActor in self?.switchToPort(portIndex) }
                }
            }
            globalHotKeys?.register(portCount: switchState?.ports.count ?? Self.defaultPortCount)
        } else {
            globalHotKeys?.unregisterAll()
        }
    }

    private func settingsDidSave() {
        kvmClient.forgetSession()
        applyHotKeySetting()
        Task { await refreshState() }
    }

    private func testConnection() async -> String {
        kvmClient.forgetSession()
        await refreshState()
        if let lastErrorMessage { return "Failed: \(lastErrorMessage)" }
        let portCount = switchState?.ports.count ?? 0
        return "Connected. \(portCount) ports, active: \(activePortName() ?? "?")."
    }

    private func showSettings() {
        if settingsWindow == nil {
            let settingsView = SettingsView(
                onSave: { [weak self] in self?.settingsDidSave() },
                onTestConnection: { [weak self] in await self?.testConnection() ?? "" }
            )
            let window = NSWindow(contentViewController: NSHostingController(rootView: settingsView))
            window.title = "KVM Switcher Settings"
            window.styleMask = [.titled, .closable]
            window.isReleasedWhenClosed = false
            window.setContentSize(SettingsView.windowSize)
            window.center()
            settingsWindow = window
        }
        NSApp.activate(ignoringOtherApps: true)
        settingsWindow?.makeKeyAndOrderFront(nil)
    }
}
