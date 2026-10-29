// swift-tools-version:5.9
// NimbusNativeKit, the iOS host for Nimbus Native apps. Xcode projects made
// by `nimbus native new` use it; the Go side is built by the project's
// "Build Go" phase (`nimbus native lib ios`).
import PackageDescription

let package = Package(
    name: "NimbusNativeKit",
    platforms: [.iOS(.v15)],
    products: [.library(name: "NimbusNativeKit", targets: ["NimbusNativeKit"])],
    targets: [
        .target(name: "NimbusNativeC", path: "hosts/ios/Sources/NimbusNativeC"),
        .target(name: "NimbusNativeKit", dependencies: ["NimbusNativeC"], path: "hosts/ios/Sources/NimbusNativeKit"),
    ]
)
