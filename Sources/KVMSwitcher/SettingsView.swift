import ServiceManagement
import SwiftUI

struct SettingsView: View {
    static let editablePortCount = 4

    /// Called after Save with the new values already persisted.
    let onSave: () -> Void
    /// Returns a human-readable result of a live connection test.
    let onTestConnection: () async -> String

    @State private var hostText = AppSettings.host
    @State private var passwordText = KeychainStore.password ?? ""
    @State private var portNameTexts: [String] = (0 ..< SettingsView.editablePortCount).map {
        AppSettings.localName(forPortIndex: $0)
    }
    @State private var globalHotKeysEnabled = AppSettings.globalHotKeysEnabled
    @State private var launchAtLogin = SMAppService.mainApp.status == .enabled
    @State private var statusMessage = ""
    @State private var isTesting = false

    var body: some View {
        Form {
            Section("GL.iNet KVM") {
                TextField("Address", text: $hostText, prompt: Text("glkvm.local"))
                SecureField("Admin password", text: $passwordText)
                HStack {
                    Button("Test Connection") { testConnection() }
                        .disabled(isTesting)
                    Button("Reset Pinned Certificate") {
                        AppSettings.pinnedCertificateFingerprint = nil
                        statusMessage = "Certificate will be re-pinned on next connection."
                    }
                }
                if !statusMessage.isEmpty {
                    Text(statusMessage).font(.callout).foregroundStyle(.secondary)
                }
            }

            Section("Port names (used when the KVM has none set)") {
                ForEach(0 ..< SettingsView.editablePortCount, id: \.self) { portIndex in
                    TextField("Port \(portIndex + 1)", text: $portNameTexts[portIndex],
                              prompt: Text("Port \(portIndex + 1)"))
                }
            }

            Section("General") {
                Toggle("Global hotkeys (\(GlobalHotKeys.modifierDescription)1 – 4)", isOn: $globalHotKeysEnabled)
                Toggle("Launch at login", isOn: $launchAtLogin)
            }

            HStack {
                Spacer()
                Button("Save") { save() }
                    .keyboardShortcut(.defaultAction)
            }
        }
        .formStyle(.grouped)
        .frame(width: 420)
    }

    private func persist() {
        AppSettings.host = hostText
        KeychainStore.password = passwordText
        AppSettings.localPortNames = portNameTexts
        AppSettings.globalHotKeysEnabled = globalHotKeysEnabled
    }

    private func save() {
        persist()
        applyLaunchAtLogin()
        onSave()
        statusMessage = "Saved."
    }

    private func testConnection() {
        persist()
        onSave()
        isTesting = true
        statusMessage = "Connecting…"
        Task {
            statusMessage = await onTestConnection()
            isTesting = false
        }
    }

    private func applyLaunchAtLogin() {
        do {
            if launchAtLogin, SMAppService.mainApp.status != .enabled {
                try SMAppService.mainApp.register()
            } else if !launchAtLogin, SMAppService.mainApp.status == .enabled {
                try SMAppService.mainApp.unregister()
            }
        } catch {
            statusMessage = "Launch at login failed: \(error.localizedDescription)"
        }
    }
}
