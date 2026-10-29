# Nimbus Native

Native iOS and Android apps written in Go — the app's code is compiled into
the app and drives real native views (UIKit, Android Views). Nimbus is the
server; Nimbus Native is the device.

**Status: Phase 0e** — apps are Go components rendered as real UIKit and
Android views, laid out by a pure-Go flexbox engine, over our own
toolchain, bridge and typed wire format — with hot reload while you work.

```go
type Counter struct{}

func (Counter) Render(c *ui.Ctx) *ui.Element {
	n, setN := ui.State(c, 0)
	return ui.Pressable(func() { setN(n + 1) }, ui.Style{Padding: ui.All(12)},
		ui.Text(fmt.Sprintf("Tapped %d times", n), ui.Style{FontWeight: 600}),
	)
}

func init() { ui.Mount(Counter{}) }

func main() { ui.Main() } // runs only in dev mode; on a device the host owns the process
```

## Layout

| Path | What |
|---|---|
| `bridge/include/nn.h` | The C ABI: the only boundary between Go and a host |
| `bridge/` | Go side of the ABI (batched outbox, panic recovery, handle table, stats); `jni_android.c` binds it to the Kotlin `Bridge` |
| `bridge/internal/abitest/` | Desktop C host that checks the whole nn.h contract (`go test ./bridge`) |
| `hosts/ios/` | `NimbusNativeKit` (Swift) |
| `hosts/android/` | `space.nimbusgo.nativekit` (Kotlin): `Bridge`, wire runtime, renderer |
| `wire/` | Wire format runtime (Go); `wire/schema` reads schemas, `wire/gen` generates Go, Swift, Kotlin |
| `hosts/*/…/Wire*` | Swift and Kotlin wire runtimes |
| `layout/` | Flexbox engine in pure Go; 62 fixtures match Chromium's layout; incremental |
| `ui/` | Components, hooks (State, Reducer, Effect, Memo, Ref, Viewport), elements, keyed reconciler, UI loop |
| `ui/render/` | The Go ↔ native renderer protocol (generated into both kits) |
| `hosts/ios/…/Renderer.swift` | UIKit renderer and `NimbusViewController` |
| `hosts/android/…/Renderer.kt`, `NimbusView.kt` | Android Views renderer and root view |
| `dev/` | Dev mode: `frame` (the socket protocol), `client` (the shell's proxy); the relay is in `toolchain/dev.go` |
| `toolchain/` | Builds apps with the platform tools directly — no gomobile, no Gradle; runs `gen` first |
| `cmd/nimbus-native/` | The toolchain behind `nimbus native …` (never called directly) |
| `examples/hello/` | Bridge and wire checks |
| `examples/gallery/` | Every primitive plus a 1,000-row list, with a built-in benchmark |
| `examples/baseline/` | The same list in plain Android views, to calibrate emulator numbers |

## Try it

Everything goes through the `nimbus` CLI — there is no second tool:

```sh
nimbus native doctor
nimbus native dev ios ./examples/gallery       # hot reload: edit, save, see it
nimbus native run ios ./examples/gallery       # boots a simulator if needed
nimbus native run android ./examples/gallery   # boots the first AVD if needed
nimbus native build ios|android ./examples/gallery
nimbus native gen ./examples/hello             # every build does this too
```

`nimbus native` runs the toolchain (`cmd/nimbus-native`) at the version the
app's `go.mod` requires, so the CLI always matches the app's runtime. Outside
such a module, set `NIMBUS_NATIVE_BIN` to a built toolchain.

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

`nimbus native gen` (run by every build) writes `wire_gen.go` next to the
schema (`proto.Encode`, `proto.Decode`, methods on each type) and the same
types for the hosts: `Proto.Message.hello(Proto.Hello(name: "Yash"))` in
Swift, `Proto.Hello(name = "Yash")` in Kotlin. Supported: bool, every int and uint
size, float32/64, string, []byte, lists, maps with string or integer keys,
optionals (`*T`), nested structs, recursion through lists (`[]*Node`),
unions, open enums.

The encoding is compact binary (varints, zigzag, little-endian floats,
sorted map keys; spec in `wire/wire.go`). Each schema has a hash; the host
passes the combined hash to `nn_start`, which refuses a mismatch with
`NN_ERR_SCHEMA`. `go test ./wire/gen` proves the three languages agree:
Go encodes every kitchen-sink fixture plus corrupt variants, and compiled
Swift and Kotlin programs must decode and re-encode them to identical bytes
and reject the same bad inputs.

## Android is Kotlin

The Android kit, generated wire types and example activities are Kotlin.
`nimbus native` downloads a pinned Kotlin compiler (2.0.21, SHA-256
checked) into the user cache on the first Android build; `NIMBUS_KOTLINC`
points it at another. Plain Java activities still build alongside.

## How a build works

- **iOS:** `go build -buildmode=c-archive` (GOOS=ios, Xcode clang) → `libapp.a`,
  then `swiftc` links it with the host and app Swift code, writes Info.plist,
  ad-hoc signs the `.app`.
- **Android:** `go build -buildmode=c-shared` (NDK clang, 16 KB pages) →
  `libnimbusapp.so` per ABI, then `kotlinc` + `d8` (with the Kotlin standard
  library) → `classes.dex`, `aapt2` →
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

## Dev mode

```
 laptop                                        simulator / emulator
 nimbus native dev ── relay :7357 ◄──TCP──►   dev shell: the real app, its Go
   │ your app, built for the laptop            library swapped for a proxy
   └ restarted on every save                   (dev/client); native code as shipped
```

`nimbus native dev ios|android` builds a dev shell once, then runs the app's
Go code on the laptop (`-tags nimbusdev`: the same bridge API over a
socket). Saving a `.go` file rebuilds and restarts it; the shell clears the
old views and the new process renders. A compile error is printed and the
last good build keeps running. A wire-schema change rebuilds the shell. The
relay caches the device's text measurements, so a reload re-measures
nothing. Android reaches the relay through `adb reverse`.

Measured on the gallery (1,000 rows): a save is on screen in **1.9–3.5 s on
the iOS simulator and 2.7 s on the Android emulator**, of which 0.6–2.3 s is
compiling — this Mac runs the Intel build of Go under Rosetta; the Apple
Silicon build compiles roughly twice as fast. App state starts fresh on each
reload.

## How a screen gets drawn

1. A state change (or host event) is queued to the UI loop goroutine.
2. Components whose props or state changed render again; the rest are kept.
3. The keyed reconciler turns the new elements into ops — create, children
   order, style, text, image, input — only for what changed.
4. The flexbox engine lays out only changed subtrees, asking the host to
   measure text (a synchronous host call, cached per text).
5. Changed frames join the ops in one `render.Commit`, applied on the UI
   thread in a single pass.

## Navigation

```go
ui.Stack("Orders", OrdersScreen{}, ui.Flex(1))   // the app's root

nav := ui.Navigation(c)                          // in any component on a screen
nav.Push("Order #10012", OrderDetail{o})
nav.Pop()
```

The stack is native: `UINavigationController` on iOS (its bar, push
animation and swipe-back), a Material-style bar with a back arrow, slide
transitions and the system back button on Android
(`onBackPressed` → `renderer.back()`). Go owns the screens; when the user
goes back, the host tells Go (`Popped`) and the two stay in step. Each
screen's content is Go-rendered at the size the host reports.

| Gallery: tap a row → detail | iOS simulator | Android emulator |
|---|---|---|
| Push, tap → committed | Go 2 ms; UIKit push 57–123 ms | 19 ms total |
| Native back → Go follows | ✓ | ✓ |

## Lists

`ui.List(count, item, style)` renders only the rows on screen and about one
screen above and below; rows can differ in height (each is measured once
shown). Scrolling renders no app components — only rows entering and
leaving — and re-windows only when the screen nears the edge of what is
rendered.

| Gallery, 1,000 rows | iOS simulator | Android emulator |
|---|---|---|
| Native views at start | 256 (was 8,014) | 256 |
| First render, Go + layout / apply | 22 ms / 84 ms (was 143 / 457) | 285 ms / 154 ms (was 1.8 s / 1 s) |
| Tap → screen median | 2.2 ms | 25 ms (was 52) |
| Fast scroll, 180 frames | 60 fps, 0 dropped, 37 window updates | 51.6 fps (was 38) |
| Native views in the list after scrolling | 36 | 39 |

## Phase 0d results

| | iOS simulator | Android emulator* |
|---|---|---|
| First render, 1,000 rows (8,014 native views) | Go 21 ms + layout 128 ms; UIKit 457 ms | Go 422 ms + layout 1.4 s; apply 1 s |
| Tap → screen (20 taps) | median **3.2 ms**, worst 5 ms | median 52 ms (frame-bound) |
| Tap commit | 4 ops; Go 0.9 ms, layout 1.7 ms | 1 op; Go 1.7 ms, layout 3.8 ms |
| Scroll through the list, 180 frames | **60 fps, 0 dropped** | 38 fps — plain Android views: 37 fps |
| Typing → Go → greeting updates | ✓ | ✓ |
| Layout of the 1,000-row screen, desktop | full 4.7 ms; one change 0.08 ms | |

\* The emulator is the limit here: the same list built with plain Android
views scrolls at the same rate. First render is slow because all 1,000 rows
exist as native views and measure their text; a virtualized list (Phase 1)
creates only the visible rows.

## Phase 0c results

| | Desktop | iOS simulator | Android emulator* |
|---|---|---|---|
| Swift / Kotlin byte-identical to Go (all fixtures + corrupt inputs) | ✓ | — | — |
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
