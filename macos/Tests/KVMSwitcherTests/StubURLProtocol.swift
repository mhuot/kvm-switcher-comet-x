import Foundation

/// Answers URLSession requests from a test-supplied handler instead of the network.
final class StubURLProtocol: URLProtocol {
    struct StubResponse {
        let statusCode: Int
        let body: Data
    }

    /// Request plus its body, which URLSession moves into a stream before the protocol sees it.
    struct RecordedRequest {
        let request: URLRequest
        let body: Data
    }

    private static let lock = NSLock()
    nonisolated(unsafe) private static var requestHandler: ((URLRequest) -> StubResponse)?
    nonisolated(unsafe) private static var recorded: [RecordedRequest] = []

    static var recordedRequests: [RecordedRequest] {
        lock.withLock { recorded }
    }

    static func install(_ handler: @escaping (URLRequest) -> StubResponse) {
        lock.withLock {
            requestHandler = handler
            recorded = []
        }
    }

    static func sessionConfiguration() -> URLSessionConfiguration {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubURLProtocol.self]
        return configuration
    }

    override class func canInit(with request: URLRequest) -> Bool { true }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let requestBody = request.httpBody ?? Self.readAll(request.httpBodyStream)
        let handler = Self.lock.withLock { () -> ((URLRequest) -> StubResponse)? in
            Self.recorded.append(RecordedRequest(request: request, body: requestBody))
            return Self.requestHandler
        }
        guard let handler, let requestURL = request.url else {
            client?.urlProtocol(self, didFailWithError: URLError(.badServerResponse))
            return
        }

        let stubResponse = handler(request)
        let httpResponse = HTTPURLResponse(
            url: requestURL, statusCode: stubResponse.statusCode, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"]
        )!
        client?.urlProtocol(self, didReceive: httpResponse, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: stubResponse.body)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}

    private static func readAll(_ bodyStream: InputStream?) -> Data {
        guard let bodyStream else { return Data() }
        bodyStream.open()
        defer { bodyStream.close() }
        var collectedBody = Data()
        var readBuffer = [UInt8](repeating: 0, count: 4096)
        while bodyStream.hasBytesAvailable {
            let bytesRead = bodyStream.read(&readBuffer, maxLength: readBuffer.count)
            if bytesRead <= 0 { break }
            collectedBody.append(readBuffer, count: bytesRead)
        }
        return collectedBody
    }
}
