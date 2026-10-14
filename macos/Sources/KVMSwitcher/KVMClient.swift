import CryptoKit
import Foundation

/// One downstream port on the GL.iNet KVM's built-in switch.
struct KVMPort {
    let portIndex: Int
    let portId: String
    let deviceName: String
    let hasVideoLink: Bool

    var displayName: String {
        if !deviceName.isEmpty { return deviceName }
        let localName = AppSettings.localName(forPortIndex: portIndex)
        return localName.isEmpty ? "Port \(portIndex + 1)" : localName
    }
}

struct KVMSwitchState {
    let activePortIndex: Int
    let ports: [KVMPort]
}

enum KVMError: LocalizedError {
    case notConfigured
    case loginRejected
    case twoFactorRequired
    case certificateChanged
    case apiError(String)
    case unexpectedResponse

    var errorDescription: String? {
        switch self {
        case .notConfigured: return "Set the KVM address and password in Settings."
        case .loginRejected: return "Login rejected. Check the password in Settings."
        case .twoFactorRequired: return "2FA is enabled on the KVM; this app does not support it."
        case .certificateChanged:
            return "The KVM's certificate changed. Reset the pinned certificate in Settings if expected."
        case let .apiError(message): return "KVM error: \(message)"
        case .unexpectedResponse: return "Unexpected response from the KVM."
        }
    }
}

/// Talks to the GLKVM (PiKVM-derived kvmd) HTTP API.
///
/// Endpoints, from the web UI bundle and a HAR capture:
///   POST /api/auth/login        form: user=admin&passwd=…   -> { token }
///   GET  /api/switch            header token: …             -> switch state
///   POST /api/switch/set_active?port=N  (N is zero-based)
@MainActor
final class KVMClient: NSObject, URLSessionDelegate {
    private static let adminUserName = "admin"

    private var sessionToken: String?
    /// Set from the URLSession delegate queue, read on the main actor.
    private nonisolated let certificateMismatchFlag = LockedFlag()
    private let sessionConfiguration: URLSessionConfiguration
    private let passwordProvider: () -> String?
    private lazy var urlSession: URLSession = {
        sessionConfiguration.timeoutIntervalForRequest = 8
        return URLSession(configuration: sessionConfiguration, delegate: self, delegateQueue: nil)
    }()

    /// Both parameters exist so tests can stub the network and the Keychain.
    init(
        sessionConfiguration: URLSessionConfiguration = .ephemeral,
        passwordProvider: @escaping () -> String? = { KeychainStore.password }
    ) {
        self.sessionConfiguration = sessionConfiguration
        self.passwordProvider = passwordProvider
        super.init()
    }

    func forgetSession() {
        sessionToken = nil
    }

    func fetchSwitchState() async throws -> KVMSwitchState {
        let responseData = try await authorizedRequest(method: "GET", path: "/api/switch")
        // Names are cosmetic, so a failed config read should not block switching.
        let webUIPortNames = (try? await fetchWebUIPortNames()) ?? []
        return try Self.parseSwitchState(responseData, webUIPortNames: webUIPortNames)
    }

    /// The GL.iNet web UI saves port names in its own config, not on the kvmd switch ports.
    private func fetchWebUIPortNames() async throws -> [String] {
        let responseData = try await authorizedRequest(method: "GET", path: "/api/system/get_config")
        let configResult = try JSONSerialization.jsonObject(with: responseData) as? [String: Any]
        let webUIConfig = configResult?["config"] as? [String: Any]
        return webUIConfig?["port_names"] as? [String] ?? []
    }

    func setActivePort(_ portIndex: Int) async throws {
        _ = try await authorizedRequest(
            method: "POST",
            path: "/api/switch/set_active",
            queryItems: [URLQueryItem(name: "port", value: String(portIndex))]
        )
    }

    // MARK: - Request plumbing

    private func baseURL() throws -> URL {
        let host = AppSettings.host
        guard !host.isEmpty, let url = URL(string: "https://\(host)") else {
            throw KVMError.notConfigured
        }
        return url
    }

    private func makeRequest(method: String, path: String, queryItems: [URLQueryItem] = []) throws -> URLRequest {
        var components = URLComponents(url: try baseURL(), resolvingAgainstBaseURL: false)!
        components.path = path
        if !queryItems.isEmpty { components.queryItems = queryItems }
        var request = URLRequest(url: components.url!)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        return request
    }

    /// Sends a request with the cached token, logging in first or again after a 401/403.
    private func authorizedRequest(method: String, path: String, queryItems: [URLQueryItem] = []) async throws -> Data {
        if sessionToken == nil { sessionToken = try await login() }

        var request = try makeRequest(method: method, path: path, queryItems: queryItems)
        request.setValue(sessionToken, forHTTPHeaderField: "token")
        var (responseData, statusCode) = try await send(request)

        if statusCode == 401 || statusCode == 403 {
            sessionToken = try await login()
            request.setValue(sessionToken, forHTTPHeaderField: "token")
            (responseData, statusCode) = try await send(request)
        }
        return try Self.unwrapResult(responseData, statusCode: statusCode)
    }

    private func login() async throws -> String {
        guard let password = passwordProvider(), !password.isEmpty else { throw KVMError.notConfigured }

        var request = try makeRequest(method: "POST", path: "/api/auth/login")
        request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        let formBody = "user=\(Self.formEncoded(Self.adminUserName))&passwd=\(Self.formEncoded(password))"
        request.httpBody = Data(formBody.utf8)

        let (responseData, statusCode) = try await send(request)
        if statusCode == 401 || statusCode == 403 { throw KVMError.loginRejected }
        let loginResult = try Self.unwrapResult(responseData, statusCode: statusCode)
        guard let resultObject = try? JSONSerialization.jsonObject(with: loginResult) as? [String: Any] else {
            throw KVMError.unexpectedResponse
        }
        if let token = resultObject["token"] as? String, !token.isEmpty { return token }
        if resultObject["need_2fa"] != nil || resultObject["2fa"] != nil { throw KVMError.twoFactorRequired }
        throw KVMError.unexpectedResponse
    }

    /// Strict form encoding so passwords containing "&", "+", "=" etc. survive intact.
    private static func formEncoded(_ rawValue: String) -> String {
        let unreservedCharacters = CharacterSet(
            charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
        )
        return rawValue.addingPercentEncoding(withAllowedCharacters: unreservedCharacters) ?? rawValue
    }

    private func send(_ request: URLRequest) async throws -> (Data, Int) {
        certificateMismatchFlag.value = false
        do {
            let (responseData, response) = try await urlSession.data(for: request)
            let statusCode = (response as? HTTPURLResponse)?.statusCode ?? 0
            return (responseData, statusCode)
        } catch {
            if certificateMismatchFlag.value { throw KVMError.certificateChanged }
            throw error
        }
    }

    /// kvmd wraps every reply as {"ok": Bool, "result": …}; returns the re-serialized "result".
    private static func unwrapResult(_ responseData: Data, statusCode: Int) throws -> Data {
        guard let envelope = try? JSONSerialization.jsonObject(with: responseData) as? [String: Any] else {
            throw statusCode >= 400 ? KVMError.apiError("HTTP \(statusCode)") : KVMError.unexpectedResponse
        }
        let resultValue = envelope["result"] ?? [String: Any]()
        guard envelope["ok"] as? Bool == true else {
            let errorMessage = (resultValue as? [String: Any])?["error_msg"] as? String ?? "HTTP \(statusCode)"
            throw KVMError.apiError(errorMessage)
        }
        return try JSONSerialization.data(withJSONObject: resultValue)
    }

    private static func parseSwitchState(_ resultData: Data, webUIPortNames: [String]) throws -> KVMSwitchState {
        guard
            let switchResult = try JSONSerialization.jsonObject(with: resultData) as? [String: Any],
            let summary = switchResult["summary"] as? [String: Any],
            let activePortIndex = summary["active_port"] as? Int,
            let model = switchResult["model"] as? [String: Any],
            let rawPorts = model["ports"] as? [[String: Any]]
        else { throw KVMError.unexpectedResponse }

        let videoLinks = (switchResult["video"] as? [String: Any])?["links"] as? [Bool] ?? []
        let ports = rawPorts.enumerated().map { portIndex, rawPort in
            let switchPortName = (rawPort["name"] as? String ?? "").trimmingCharacters(in: .whitespaces)
            let webUIPortName = portIndex < webUIPortNames.count
                ? webUIPortNames[portIndex].trimmingCharacters(in: .whitespaces) : ""
            return KVMPort(
                portIndex: portIndex,
                portId: rawPort["id"] as? String ?? "\(portIndex + 1)",
                deviceName: switchPortName.isEmpty ? webUIPortName : switchPortName,
                hasVideoLink: portIndex < videoLinks.count ? videoLinks[portIndex] : true
            )
        }
        return KVMSwitchState(activePortIndex: activePortIndex, ports: ports)
    }

    // MARK: - TLS: trust-on-first-use pinning of the KVM's self-signed certificate

    nonisolated func urlSession(
        _ session: URLSession,
        didReceive challenge: URLAuthenticationChallenge,
        completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void
    ) {
        guard challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodServerTrust,
              challenge.protectionSpace.host == AppSettings.host,
              let serverTrust = challenge.protectionSpace.serverTrust,
              let certificateChain = SecTrustCopyCertificateChain(serverTrust) as? [SecCertificate],
              let leafCertificate = certificateChain.first
        else {
            completionHandler(.performDefaultHandling, nil)
            return
        }

        let certificateData = SecCertificateCopyData(leafCertificate) as Data
        let fingerprint = SHA256.hash(data: certificateData).map { String(format: "%02x", $0) }.joined()

        if let pinnedFingerprint = AppSettings.pinnedCertificateFingerprint {
            guard pinnedFingerprint == fingerprint else {
                certificateMismatchFlag.value = true
                completionHandler(.cancelAuthenticationChallenge, nil)
                return
            }
        } else {
            AppSettings.pinnedCertificateFingerprint = fingerprint
        }
        completionHandler(.useCredential, URLCredential(trust: serverTrust))
    }
}

/// Tiny thread-safe Bool for handing state from the URLSession delegate queue back to the client.
final class LockedFlag: @unchecked Sendable {
    private let lock = NSLock()
    private var storedValue = false

    var value: Bool {
        get { lock.withLock { storedValue } }
        set { lock.withLock { storedValue = newValue } }
    }
}
