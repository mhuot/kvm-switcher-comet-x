import Foundation
@testable import KVMSwitcher
import XCTest

/// Exercises KVMClient against the shared fixtures in <repo>/fixtures (see docs/API.md).
@MainActor
final class KVMClientTests: XCTestCase {
    private static let testHost = "kvm.test"
    nonisolated private static let testPassword = "correct horse"
    private static let fixturesDirectory = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent() // KVMSwitcherTests
        .deletingLastPathComponent() // Tests
        .deletingLastPathComponent() // macos
        .deletingLastPathComponent() // repo root
        .appendingPathComponent("fixtures")

    private var testDefaultsSuiteName = ""

    override func setUp() async throws {
        testDefaultsSuiteName = "KVMSwitcherTests-\(UUID().uuidString)"
        AppSettings.defaults = UserDefaults(suiteName: testDefaultsSuiteName)!
        AppSettings.host = Self.testHost
    }

    override func tearDown() async throws {
        AppSettings.defaults.removePersistentDomain(forName: testDefaultsSuiteName)
        AppSettings.defaults = .standard
    }

    // MARK: - Helpers

    private static func fixture(_ fileName: String) throws -> Data {
        try Data(contentsOf: fixturesDirectory.appendingPathComponent(fileName))
    }

    private func makeClient(password: String? = KVMClientTests.testPassword) -> KVMClient {
        KVMClient(sessionConfiguration: StubURLProtocol.sessionConfiguration(), passwordProvider: { password })
    }

    /// Routes requests by path to fixture files; `overrides` replaces specific paths.
    private func installKVM(
        switchFixture: String = "switch_state.json",
        configFixture: String = "get_config.json",
        overrides: [String: (Int, String)] = [:]
    ) {
        let routes: [String: (Int, String)] = [
            "/api/auth/login": (200, "login_success.json"),
            "/api/switch": (200, switchFixture),
            "/api/system/get_config": (200, configFixture),
            "/api/switch/set_active": (200, "set_active_success.json"),
        ].merging(overrides) { _, override in override }

        StubURLProtocol.install { request in
            guard let (statusCode, fixtureName) = routes[request.url?.path ?? ""],
                  let body = try? Self.fixture(fixtureName)
            else { return .init(statusCode: 404, body: Data()) }
            return .init(statusCode: statusCode, body: body)
        }
    }

    private func requests(toPath path: String) -> [StubURLProtocol.RecordedRequest] {
        StubURLProtocol.recordedRequests.filter { $0.request.url?.path == path }
    }

    // MARK: - Shared contract cases

    func testSharedFixtureCases() async throws {
        struct ExpectedPort: Decodable {
            let port_index: Int
            let display_name: String
            let has_video_link: Bool
        }
        struct ExpectedState: Decodable {
            let active_port_index: Int
            let ports: [ExpectedPort]
        }
        struct ContractCase: Decodable {
            let name: String
            let `switch`: String
            let config: String
            let expected: ExpectedState
        }
        struct ContractFile: Decodable {
            let cases: [ContractCase]
        }

        let contract = try JSONDecoder().decode(ContractFile.self, from: Self.fixture("cases.json"))
        XCTAssertFalse(contract.cases.isEmpty)
        for contractCase in contract.cases {
            installKVM(switchFixture: contractCase.switch, configFixture: contractCase.config)
            let switchState = try await makeClient().fetchSwitchState()

            XCTAssertEqual(switchState.activePortIndex, contractCase.expected.active_port_index, contractCase.name)
            XCTAssertEqual(switchState.ports.map(\.displayName),
                           contractCase.expected.ports.map(\.display_name), contractCase.name)
            XCTAssertEqual(switchState.ports.map(\.hasVideoLink),
                           contractCase.expected.ports.map(\.has_video_link), contractCase.name)
        }
    }

    // MARK: - Authentication

    func testLogsInOnceAndSendsTokenHeader() async throws {
        installKVM()
        let client = makeClient()
        _ = try await client.fetchSwitchState()
        _ = try await client.fetchSwitchState()

        XCTAssertEqual(requests(toPath: "/api/auth/login").count, 1)
        let switchRequests = requests(toPath: "/api/switch")
        XCTAssertEqual(switchRequests.count, 2)
        XCTAssertEqual(switchRequests[0].request.value(forHTTPHeaderField: "token"), String(repeating: "0", count: 64))
    }

    func testLoginFormEncodesSpecialCharacters() async throws {
        installKVM()
        _ = try await makeClient(password: "p&ss+word= ü").fetchSwitchState()

        let loginRequest = try XCTUnwrap(requests(toPath: "/api/auth/login").first)
        XCTAssertEqual(loginRequest.request.httpMethod, "POST")
        XCTAssertEqual(String(data: loginRequest.body, encoding: .utf8),
                       "user=admin&passwd=p%26ss%2Bword%3D%20%C3%BC")
    }

    func testExpiredTokenTriggersOneReloginAndRetry() async throws {
        var switchCallCount = 0
        let switchStateBody = try Self.fixture("switch_state.json")
        let unauthorizedBody = try Self.fixture("error_unauthorized.json")
        let loginBody = try Self.fixture("login_success.json")
        let configBody = try Self.fixture("get_config.json")
        StubURLProtocol.install { request in
            switch request.url?.path {
            case "/api/auth/login": return .init(statusCode: 200, body: loginBody)
            case "/api/system/get_config": return .init(statusCode: 200, body: configBody)
            case "/api/switch":
                switchCallCount += 1
                return switchCallCount == 1
                    ? .init(statusCode: 401, body: unauthorizedBody)
                    : .init(statusCode: 200, body: switchStateBody)
            default: return .init(statusCode: 404, body: Data())
            }
        }

        let switchState = try await makeClient().fetchSwitchState()
        XCTAssertEqual(switchState.activePortIndex, 2)
        XCTAssertEqual(requests(toPath: "/api/auth/login").count, 2)
    }

    func testWrongPasswordReportsLoginRejected() async {
        installKVM(overrides: ["/api/auth/login": (403, "error_forbidden.json")])
        do {
            _ = try await makeClient().fetchSwitchState()
            XCTFail("Expected loginRejected")
        } catch KVMError.loginRejected {
            // expected
        } catch {
            XCTFail("Unexpected error: \(error)")
        }
    }

    func testMissingPasswordReportsNotConfigured() async {
        installKVM()
        do {
            _ = try await makeClient(password: nil).fetchSwitchState()
            XCTFail("Expected notConfigured")
        } catch KVMError.notConfigured {
            XCTAssertTrue(StubURLProtocol.recordedRequests.isEmpty)
        } catch {
            XCTFail("Unexpected error: \(error)")
        }
    }

    // MARK: - Switching and names

    func testSetActivePortSendsZeroBasedIndex() async throws {
        installKVM()
        try await makeClient().setActivePort(3)

        let setActiveRequest = try XCTUnwrap(requests(toPath: "/api/switch/set_active").first)
        XCTAssertEqual(setActiveRequest.request.httpMethod, "POST")
        XCTAssertEqual(setActiveRequest.request.url?.query, "port=3")
    }

    func testConfigFailureFallsBackToDefaultNames() async throws {
        installKVM(overrides: ["/api/system/get_config": (500, "error_forbidden.json")])
        let switchState = try await makeClient().fetchSwitchState()
        XCTAssertEqual(switchState.ports.map(\.displayName), ["Port 1", "Port 2", "Port 3", "Port 4"])
    }

    func testLocalNamesFillGapsButKVMNamesWin() async throws {
        installKVM(configFixture: "get_config_without_port_names.json")
        AppSettings.localPortNames = ["Local One", "", "Local Three"]
        let switchState = try await makeClient().fetchSwitchState()
        XCTAssertEqual(switchState.ports.map(\.displayName), ["Local One", "Port 2", "Local Three", "Port 4"])

        installKVM(configFixture: "get_config.json")
        let namedState = try await makeClient().fetchSwitchState()
        XCTAssertEqual(namedState.ports.first?.displayName, "Workstation")
    }

    func testApiErrorMessageIsSurfaced() async {
        installKVM(overrides: ["/api/switch/set_active": (200, "error_forbidden.json")])
        do {
            try await makeClient().setActivePort(1)
            XCTFail("Expected apiError")
        } catch let KVMError.apiError(message) {
            XCTAssertEqual(message, "Forbidden")
        } catch {
            XCTFail("Unexpected error: \(error)")
        }
    }

    // MARK: - Settings

    func testNormalizedHostStripsSchemeAndSlashes() {
        XCTAssertEqual(AppSettings.normalizedHost("  https://glkvm.local/ "), "glkvm.local")
        XCTAssertEqual(AppSettings.normalizedHost("HTTP://10.0.0.5//"), "10.0.0.5")
        XCTAssertEqual(AppSettings.normalizedHost("glkvm.local"), "glkvm.local")
    }

    func testEmptyHostReportsNotConfigured() async {
        AppSettings.host = ""
        installKVM()
        do {
            _ = try await makeClient().fetchSwitchState()
            XCTFail("Expected notConfigured")
        } catch KVMError.notConfigured {
            // expected
        } catch {
            XCTFail("Unexpected error: \(error)")
        }
    }
}
