# Nimbus Native

Native iOS and Android apps written in Go — the app's code is compiled into
the app and drives real native views (UIKit, Android Views). Nimbus is the
server; Nimbus Native is the device.

**Status: Phase 0a** — own toolchain and C ABI bridge, proven on the iOS
simulator and the Android emulator. No UI layer yet.

## Layout

| Path | What |
|---|---|
| `bridge/include/nn.h` | The C ABI: the only boundary between Go and a host |
| `bridge/` | Go side of the ABI; `jni_android.c` binds it to Java |
| `hosts/ios/` | `NimbusNativeKit` (Swift) |
| `hosts/android/` | `space.nimbusgo.nativekit.Bridge` (Java) |
| `toolchain/` | Builds apps with the platform tools directly — no gomobile, no Gradle |
| `cmd/nimbus-native/` | CLI (becomes `nimbus native …`) |
| `examples/hello/` | Phase 0a round-trip app |

## Try it

```sh
go build -o bin/nimbus-native ./cmd/nimbus-native
./bin/nimbus-native doctor
./bin/nimbus-native run ios ./examples/hello       # boots a simulator if needed
./bin/nimbus-native run android ./examples/hello   # boots the first AVD if needed
```

`run` streams the app's log until you press Ctrl+C.

## How a build works

- **iOS:** `go build -buildmode=c-archive` (GOOS=ios, Xcode clang) → `libapp.a`,
  then `swiftc` links it with the host and app Swift code, writes Info.plist,
  ad-hoc signs the `.app`.
- **Android:** `go build -buildmode=c-shared` (NDK clang, 16 KB pages) →
  `libnimbusapp.so` per ABI, then `javac` + `d8` → `classes.dex`, `aapt2` →
  APK, native libs stored uncompressed, `zipalign -P 16`, `apksigner`.

## Phase 0a results (M1, Go 1.23.3)

| | iOS simulator | Android emulator (API 34, arm64) |
|---|---|---|
| Runtime start | 0.1 ms | 21.3 ms |
| Sync call native → Go | 0.71 µs | 1.83 µs |
| Async ping-pong, incl. main-thread hop | 104 µs | 979 µs |
| Go panic recovered, app keeps running | ✓ | ✓ |
| Go goroutines → native | ✓ | ✓ |
| 16 KB page alignment | — | ✓ (zip + ELF) |
