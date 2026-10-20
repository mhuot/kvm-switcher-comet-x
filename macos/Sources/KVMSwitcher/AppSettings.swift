import Foundation

/// What the menu bar shows next to the icon for the active port.
enum MenuBarLabelStyle: String, CaseIterable, Identifiable {
    case portName
    case iconOnly

    var id: String { rawValue }

    var settingsTitle: String {
        switch self {
        case .portName: return "Port name"
        case .iconOnly: return "Icon only"
        }
    }
}

/// Persisted, non-secret preferences. The password lives in the Keychain (see `KeychainStore`).
enum AppSettings {
    /// Replaceable so tests can use a throwaway suite instead of the app's real preferences.
    static var defaults = UserDefaults.standard

    private enum Key {
        static let host = "kvmHost"
        static let portNames = "localPortNames"
        static let globalHotKeysEnabled = "globalHotKeysEnabled"
        static let menuBarLabelStyle = "menuBarLabelStyle"
        static let pinnedCertificateFingerprint = "pinnedCertificateFingerprint"
    }

    /// Bare host name or IP of the KVM, e.g. "glkvm.local".
    static var host: String {
        get { defaults.string(forKey: Key.host) ?? "" }
        set { defaults.set(normalizedHost(newValue), forKey: Key.host) }
    }

    /// Local fallback names for ports, used when the device reports an empty name.
    static var localPortNames: [String] {
        get { defaults.stringArray(forKey: Key.portNames) ?? [] }
        set { defaults.set(newValue, forKey: Key.portNames) }
    }

    static var globalHotKeysEnabled: Bool {
        get { defaults.object(forKey: Key.globalHotKeysEnabled) as? Bool ?? true }
        set { defaults.set(newValue, forKey: Key.globalHotKeysEnabled) }
    }

    static var menuBarLabelStyle: MenuBarLabelStyle {
        get {
            let storedStyle = defaults.string(forKey: Key.menuBarLabelStyle)
            // "portNumber" was removed once the icon itself showed the number.
            if storedStyle == "portNumber" { return .iconOnly }
            return storedStyle.flatMap(MenuBarLabelStyle.init) ?? .portName
        }
        set { defaults.set(newValue.rawValue, forKey: Key.menuBarLabelStyle) }
    }

    /// SHA-256 of the KVM's self-signed leaf certificate, pinned on first connection.
    static var pinnedCertificateFingerprint: String? {
        get { defaults.string(forKey: Key.pinnedCertificateFingerprint) }
        set { defaults.set(newValue, forKey: Key.pinnedCertificateFingerprint) }
    }

    static func localName(forPortIndex portIndex: Int) -> String {
        let names = localPortNames
        guard portIndex < names.count else { return "" }
        return names[portIndex].trimmingCharacters(in: .whitespaces)
    }

    /// Accepts "https://1.2.3.4/", "1.2.3.4" etc. and returns just the host part.
    static func normalizedHost(_ rawHost: String) -> String {
        var trimmedHost = rawHost.trimmingCharacters(in: .whitespacesAndNewlines)
        for scheme in ["https://", "http://"] where trimmedHost.lowercased().hasPrefix(scheme) {
            trimmedHost = String(trimmedHost.dropFirst(scheme.count))
        }
        while trimmedHost.hasSuffix("/") {
            trimmedHost = String(trimmedHost.dropLast())
        }
        return trimmedHost
    }
}
