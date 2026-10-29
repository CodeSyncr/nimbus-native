# Nimbus Native

Native iOS and Android apps written in Go — the app's code is compiled into
the app and drives real native views (UIKit, Android Views). Nimbus is the
server; Nimbus Native is the device.

**Status: Phase 0c** — own toolchain, a batched C ABI bridge, and a typed
wire format generated for Go, Swift and Java from one Go schema. Proven on
the iOS simulator, the Android emulator and the desktop. No UI layer yet.

## Layout

| Path | What |
|---|---|
| `bridge/include/nn.h` | The C ABI: the only boundary between Go and a host |
| `bridge/` | Go side of the ABI (batched outbox, panic recovery, handle table, stats); `jni_android.c` binds it to Java |
| `bridge/internal/abitest/` | Desktop C host that checks the whole nn.h contract (`go test ./bridge`) |
| `hosts/ios/` | `NimbusNativeKit` (Swift) |
| `hosts/android/` | `space.nimbusgo.nativekit.Bridge` (Java) |
| `wire/` | Wire format runtime (Go); `wire/schema` reads schemas, `wire/gen` generates Go, Swift, Java |
| `hosts/*/…/Wire*` | Swift and Java wire runtimes |
| `toolchain/` | Builds apps with the platform tools directly — no gomobile, no Gradle; runs `gen` first |
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

## Typed messages: the wire schema

An app describes what crosses the bridge as plain Go types in a package of
its own (the hello app uses `examples/hello/proto`):

```go
//wire:message
type Hello struct{ Name string }

//wire:enum
type Kind uint8

//wire:union
type Op interface{ isOp() }

//wire:variant Op
type Remove struct{ ID uint32 }
```

`nimbus-native gen` (run by every build) writes `wire_gen.go` next to the
schema (`proto.Encode`, `proto.Decode`, methods on each type) and the same
types for the hosts: `Proto.Message.hello(Proto.Hello(name: "Yash"))` in
Swift, `new Proto.Hello()` in Java. Supported: bool, every int and uint
size, float32/64, string, []byte, lists, maps with string or integer keys,
optionals (`*T`), nested structs, recursion through lists (`[]*Node`),
unions, open enums.

The encoding is compact binary (varints, zigzag, little-endian floats,
sorted map keys; spec in `wire/wire.go`). Each schema has a hash; the host
passes the combined hash to `nn_start`, which refuses a mismatch with
`NN_ERR_SCHEMA`. `go test ./wire/gen` proves the three languages agree:
Go encodes every kitchen-sink fixture plus corrupt variants, and compiled
Swift and Java programs must decode and re-encode them to identical bytes
and reject the same bad inputs.

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

## Phase 0c results

| | Desktop | iOS simulator | Android emulator* |
|---|---|---|---|
| Swift / Java byte-identical to Go (all fixtures + corrupt inputs) | ✓ | — | — |
| Stale schema refused at start | ✓ | ✓ | ✓ |
| Typed sync call (encode + call + decode) | — | 0.67 µs | ~17 µs |
| 1,000-row list (strings, optional, enum, tags, map) Go → host | — | 1.66 µs/row | ~15 µs/row |
| 1,000-node tree, Go encode / decode | 0.12 / 0.30 ms | — | — |
| 100k typed messages, in order | — | 7 drains | 2 drains |
| Leak check (2 soak rounds) | — | heap 871 → 871 KB | heap 665 → 665 KB |

\* Emulator timings are not trustworthy (see below); a real device is next.

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
