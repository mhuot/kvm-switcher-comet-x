// swift-tools-version:5.10
import PackageDescription

let package = Package(
    name: "KVMSwitcher",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(
            name: "KVMSwitcher",
            path: "Sources/KVMSwitcher",
            linkerSettings: [.linkedFramework("Carbon")]
        )
    ]
)
