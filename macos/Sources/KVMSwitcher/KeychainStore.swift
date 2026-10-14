import Foundation
import Security

/// Minimal generic-password wrapper for the KVM admin password.
enum KeychainStore {
    private static let serviceName = "net.mhuot.KVMSwitcher"
    private static let accountName = "glkvm-admin"

    static var password: String? {
        get { readPassword() }
        set {
            if let newPassword = newValue, !newPassword.isEmpty {
                savePassword(newPassword)
            } else {
                deletePassword()
            }
        }
    }

    private static var baseQuery: [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: serviceName,
            kSecAttrAccount as String: accountName,
        ]
    }

    private static func readPassword() -> String? {
        var query = baseQuery
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var resultItem: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &resultItem) == errSecSuccess,
              let passwordData = resultItem as? Data
        else { return nil }
        return String(data: passwordData, encoding: .utf8)
    }

    private static func savePassword(_ newPassword: String) {
        let passwordData = Data(newPassword.utf8)
        let updateStatus = SecItemUpdate(
            baseQuery as CFDictionary,
            [kSecValueData as String: passwordData] as CFDictionary
        )
        if updateStatus == errSecItemNotFound {
            var addQuery = baseQuery
            addQuery[kSecValueData as String] = passwordData
            addQuery[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
            SecItemAdd(addQuery as CFDictionary, nil)
        }
    }

    private static func deletePassword() {
        SecItemDelete(baseQuery as CFDictionary)
    }
}
