// swift-tools-version:5.9
// NimbusNativeKit, the iOS host for Nimbus Native apps, as a Swift package.
// Projects made by `nimbus native new` hold a copy of it (native/ios/
// NimbusNativeKit) that `nimbus native lib ios` keeps at the go.mod version.
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
