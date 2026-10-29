# Nimbus Native

Native iOS and Android apps written in Go — the app's code is compiled into
the app and drives real native views (UIKit, Android Views). Nimbus is the
server; Nimbus Native is the device.

**Status: Phase 0b** — own toolchain and a hardened, batched C ABI bridge,
proven on the iOS simulator, the Android emulator and a desktop C host.
No UI layer yet.

## Layout

| Path | What |
|---|---|
| `bridge/include/nn.h` | The C ABI: the only boundary between Go and a host |
| `bridge/` | Go side of the ABI (batched outbox, panic recovery, handle table, stats); `jni_android.c` binds it to Java |
| `bridge/internal/abitest/` | Desktop C host that checks the whole nn.h contract (`go test ./bridge`) |
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

## How the bridge moves messages

- **Native → Go:** `nn_send` (async, in order) or `nn_call` (sync, returns a
  reply). A Go panic in `nn_call` comes back as `NN_ERR_PANIC` with the panic
  text and Go stack — `BridgeError` on iOS, `BridgeException` on Android.
- **Go → native:** `bridge.Send` appends to an outbox and never blocks. Only
  when the outbox goes from empty to non-empty does Go call the host's
  `wake`; the host then calls `nn_drain` on its UI thread and gets every
  waiting message in one buffer. A burst costs one thread hop, not one per
  message.
- Hosts refer to Go values by integer `Handle`, never by pointer.

## Phase 0b results

| | Desktop C host | iOS simulator | Android emulator* |
|---|---|---|---|
| 100k-message burst from Go | 362 msgs/drain (8 goroutines) | 10 drains, 99 ns/msg | 2 drains |
| Ordering kept | ✓ (per goroutine) | ✓ | ✓ |
| Sync call | 0.48 µs | 0.42 µs | 4–10 µs |
| Ping-pong round trip | 10 µs | 73 µs | 1.5–2.4 ms |
| Panic text + Go stack reach native | ✓ | ✓ | ✓ |
| Leak check: 2 soak rounds (100k calls + 100k msgs each) | heap +40 KB | heap 871 → 871 KB | heap 980 → 981 KB |

\* The emulator was under heavy load (the 0a build measured 14–19 µs per
sync call in the same session). Run back to back, 0b was no slower than
0a. Real Android latency needs a device.

## Phase 0a results (M1, Go 1.23.3)

| | iOS simulator | Android emulator (API 34, arm64) |
|---|---|---|
| Runtime start | 0.1 ms | 21.3 ms |
| Sync call native → Go | 0.71 µs | 1.83 µs |
| Async ping-pong, incl. main-thread hop | 104 µs | 979 µs |
| Go panic recovered, app keeps running | ✓ | ✓ |
| Go goroutines → native | ✓ | ✓ |
| 16 KB page alignment | — | ✓ (zip + ELF) |
