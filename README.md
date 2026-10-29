# Nimbus Native

Native iOS and Android apps written in Go, and the same app on the web.
Your app's Go code is compiled into the app and drives real native views —
UIKit on iOS, Android Views on Android — laid out by a flexbox engine
written in Go. [Nimbus](https://github.com/CodeSyncr/nimbus) is the server,
and serves the app's pages to browsers too. One language, one codebase,
backend included.

> **Beta (0.0.2).** Everything below runs on the iOS simulator, the Android
> emulator and in browsers; it has not shipped to a store yet, and APIs may
> still change. See [CHANGELOG.md](CHANGELOG.md) for what's in it and what
> isn't yet.

```sh
go install github.com/CodeSyncr/nimbus/cmd/nimbus@latest   # Nimbus v1.16.0 or later
nimbus native new shop
cd shop && go run .              # the server and the web app on :3333
nimbus native run ios            # in another terminal; or open native/ios in Xcode
```

```go
type Counter struct{}

func (Counter) Render(c *ui.Ctx) *ui.Element {
	n, setN := ui.State(c, 0)
	return ui.SafeArea(ui.Style{Padding: ui.All(16)},
		ui.Pressable(func() { setN(n + 1) }, ui.Style{Padding: ui.All(12)},
			ui.Text(fmt.Sprintf("Tapped %d times", n), ui.Style{FontWeight: 600}),
		),
	)
}

func init() { ui.Mount(Counter{}) }

func main() { ui.Main() } // runs only in dev mode; on a device the host owns the process
```

## Requirements

- Go 1.23+, and the `nimbus` CLI from [Nimbus](https://github.com/CodeSyncr/nimbus)
- iOS: macOS with Xcode 16+. Simulator builds work as they are. On an
  iPhone, over USB or Wi-Fi (paired in Xcode's Devices window), with
  Developer Mode on: `nimbus native run ios -device -team <TEAM ID>`
  (`-udid` picks a phone when several are reachable; the team can also come
  from `NIMBUS_IOS_TEAM` or the Xcode project).
- Android: the Android SDK with an NDK and build-tools, JDK 17+. The Kotlin
  compiler is downloaded on the first build.

`nimbus native doctor` checks all of it. Release Android builds are signed
with the debug key until you add yours to `native/android/app/build.gradle.kts`.

## Roadmap

Done: own toolchain (no gomobile), C ABI bridge, typed wire
format with Go/Swift/Kotlin generation, flexbox layout, components and
hooks, UIKit and Android renderers, hot reload, virtualized lists, stack
navigation, tabs, modals, safe areas, forms, theming and dark mode, a
typed client for Nimbus servers with data-loading hooks, `nimbus native new`
projects that open in Xcode and Android Studio, folder routing, native
plugins (secure storage, app info, links, key-value storage), switches,
multiline inputs, pull to refresh, keyboard avoidance and native dialogs,
and the same app on the web with server rendering for search engines.
Not yet: flex-wrap, animations and gestures, plugins on the web.

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
| `toolchain/` | Builds apps — no gomobile; runs `gen` first. `toolchain/template/` is what `nimbus native new` writes |
| `cmd/nimbus-native/` | The toolchain behind `nimbus native …` (never called directly) |
| `examples/hello/` | Bridge and wire checks |
| `examples/gallery/` | Every primitive plus a 1,000-row list, with a built-in benchmark |
| `examples/baseline/` | The same list in plain Android views, to calibrate emulator numbers |
| `examples/fullstack/` | A Nimbus server and app sharing one contract (its own module) |
| `api/` | Typed endpoints and a JSON client for Nimbus servers (standard library only) |
| `router/` | Path → page matching and `router.Push`; the table comes from `pages/` (`toolchain/routes.go`) |
| `plugin/` | Calls native code: `plugin.Call` to a Swift/Kotlin plugin (`hosts/*/Plugins.*`) |
| `web/` | The app's pages for browsers: server rendering, SEO tags, sitemap, and `host.js` |
| `plugins/` | Built-in plugins: `securestore`, `appinfo`, `linking`, `dialog`, and `storage` (a key-value file) |

## Start a project

```sh
nimbus native new myapp         # a counter; -template shop for the shop
cd myapp
go run .                        # the Nimbus server and the web app, on :3333
nimbus native run ios           # or open native/ios/Runner.xcodeproj in Xcode
nimbus native run android       # or open native/android in Android Studio
```

A project is one Go module, laid out by feature:

The generated app is a counter, ready to grow: one polished screen (a
goal with a progress bar, steps, the count saved between launches, light
and dark themes) on iOS, Android and the web. For a bigger starting point,
`nimbus native new -template shop shop` makes a small shop instead: a
catalog, a cart, sign-in (demo account `demo@shop.dev` / `password`),
checkout and orders, served by its own API.

A project is one Go module, laid out by feature:

```
counter/
  main.go                  the Nimbus server: the app's pages for browsers (and your API)
  app/
    app.go                 the root component; installs the theme
    mobile/main.go         the phone app's package main
  core/                    constants, theme
  features/counter/
    data/                  the counter's state (a ui.Store), saved on the device
    presentation/          the screen
  pages/page.go            the page at "/" (folder routing: pages/settings/ is /settings)
  native/
    ios/                   Xcode project (Runner + the NimbusNativeKit package)
    android/               Gradle project (app + the nimbusnativekit module)
```

The shop adds `shared/` (widgets, services) and features with `domain/`
(types and `api.Endpoint`s: the contract), `data/`, `presentation/` and
`server/` (handlers and validation, server only).

Xcode and Gradle build the hosts as usual; a build step in each runs
`nimbus native lib`, which builds `app/mobile` into the native library and
writes the generated Swift or Kotlin. It also keeps the project's copy of the
host kit at the version `go.mod` requires (after an upgrade, the first build
updates it and asks you to build again), and refuses to build a phone app
that imports Nimbus or any `server` package: that code stays on the server.

## Folder routing

Optional: a project with a `pages/` folder gets its URLs from it.

```
pages/orders/page.go             /orders
pages/orders/new/page.go         /orders/new    (beats :id)
pages/orders/_id/page.go         /orders/:id
pages/orders/_id/items/_item/    /orders/:id/items/:item
```

Each `page.go` declares a `Page` component; parameters fill fields tagged
`param` (string, int or int64; a value that doesn't parse matches nothing),
and an optional `Title()` names the screen:

```go
package order // pages/orders/_id

type Page struct {
	ID int `param:"id"`
}

func (p Page) Title() string                  { return fmt.Sprintf("Order #%d", p.ID) }
func (p Page) Render(c *ui.Ctx) *ui.Element { return ui.C(presentation.OrderScreen{ID: p.ID}) }
```

Every build regenerates `pages/routes_gen.go`, a static table: no reflection
or scanning at run time, and a missing or mistyped parameter field fails the
build with the folder named. Register it once, then navigate by path; the
page is pushed onto the screen's native stack:

```go
router.Register(pages.Routes)   // app/router.go
router.Push(c, "/orders/42")    // any screen
```

Go skips `_`-prefixed folders in `./...` patterns, so `go vet ./...` passes
over parameter pages; they are compiled through the table the app imports.

## Shared state

`ui.Store` is state the whole app shares, such as a cart or who is signed
in: one value per app on a phone, and one per browser tab on the web
(where every visitor runs in the same server, so a package variable would
be shared between them).

```go
var Cart = ui.NewStore(func() Cart { return nil })

items := Cart.Use(c)                                          // re-renders c when it changes
Cart.Update(c, func(ct Cart) Cart { return ct.Add(p) })       // from handlers, effects, actions
```

## Controls and dialogs

```go
ui.Switch(on, setOn, ui.Style{})                                   // UISwitch / Switch, theme-tinted
ui.TextInput(note, setNote, box).Multiline()                       // or FieldOptions{Multiline: true}
ui.List(n, row, ui.Flex(1)).OnRefresh(q.Loading, q.Refetch)        // pull to refresh (Scroll too)

ok, _ := dialog.Confirm(ctx, "Delete order?", "This can't be undone.", "Delete", true)
i, _ := dialog.Choose(ctx, "Share via", "Mail", "Messages")        // action sheet; -1 if cancelled
dialog.Toast(ctx, "Saved")
day, ok, _ := dialog.PickDate(ctx, dialog.Date, time.Now())        // Date, Time or DateTime
```

`SafeArea` keeps clear of the on-screen keyboard: its bottom padding grows
by however much of the keyboard reaches below it.

## The web

A project's Nimbus server also serves its pages to browsers (`main.go`
mounts `web.Handler` as the router's fallback):

- **Server rendering.** A request runs the page's components on the
  server, waits for its `UseQuery` data (up to 3 s), and answers with plain
  HTML: content, links (`router.Link`, `.Href`), and the page's `<title>`,
  description, Open Graph tags and canonical URL from its `Head()` method.
  `/sitemap.xml` lists the pages without parameters; `/robots.txt` points to it.
- **Then live.** `host.js` (8 KB, 3 KB gzipped) adopts that HTML. Taps and typing go to
  the page's UI session on the server, and what it renders comes back as DOM
  changes over Server-Sent Events. Layout is the browser's own flexbox.
- **Navigation.** On the web the shell is `app.WebLayout`, a header with a
  link per tab, instead of tabs and stacks. `router.Push` changes the page
  and the address; Back and Forward work; every page has its own URL.

```go
func (p Page) Head() router.Head {
	return router.Head{Title: fmt.Sprintf("Order #%d", p.ID), Description: "…", Image: "https://…/og.png"}
}
```

Each open tab holds a small UI session on the server, closed a minute after
the tab goes away. Native plugins answer `unavailable` on the web.

## Plugins: calling native code

Go reaches Swift and Kotlin through plugins. Built in:

```go
securestore.Set(ctx, "token", t)            // Keychain / Android Keystore
token, ok, err := securestore.Get(ctx, "token")
storage.Set(ctx, "theme", "dark")            // JSON in the app's data folder
info, _ := appinfo.Get(ctx)                  // version, locale, data and cache dirs
linking.Open(ctx, "https://example.com")     // browser, tel:, mailto:, maps
```

Your own plugin is a name and a handler in the host, and one call in Go.
Arguments and answers are JSON, so neither side needs generated code:

```swift
// native/ios/Runner/AppDelegate.swift, before the NimbusViewController
Plugins.register("battery") { call in
    UIDevice.current.isBatteryMonitoringEnabled = true
    call.resolve(["level": UIDevice.current.batteryLevel])
}
```

```kotlin
// native/android/.../MainActivity.kt, before the NimbusView
Plugins.register("battery") { call ->
    val bm = getSystemService(BATTERY_SERVICE) as android.os.BatteryManager
    call.resolve(mapOf("level" to bm.getIntProperty(android.os.BatteryManager.BATTERY_PROPERTY_CAPACITY) / 100.0))
}
```

```go
type battery struct{ Level float64 `json:"level"` }
b, err := plugin.Call[battery](ctx, "battery", "level", nil)
```

Handlers run on the main thread and answer once with `resolve` or
`reject(code, message)`; codes (`cancelled`, `denied`, `not_found`,
`unavailable`, `failed`) come back as `plugin.Error`, testable with
`plugin.IsCode`. `plugin.Call` blocks its goroutine until the answer, so
call it from `ui.UseQuery`, `ui.UseAction` or an effect, not in Render.
Plugins work unchanged under `nimbus native dev`.

On Android, `fmt.Println` and `log` output goes to logcat (tag
`NimbusNativeApp`), which `nimbus native run android` streams.

## Try the examples

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
such a module it runs the latest release, or `NIMBUS_NATIVE_BIN` if set.

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

Projects build with xcodebuild and Gradle (the IDEs' own builds). The
examples build with the platform tools directly:

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
Silicon build compiles roughly twice as fast.

In a project, `nimbus native dev ios|android` builds the shell with the
project's own Xcode or Gradle build and watches the whole module (not
`native/` or `build/`), so a new page folder is live on the next save.
Component state starts fresh on each reload, but the screen doesn't: the
selected tabs and every screen pushed with `router.Push` come back. Each
`dev` session starts on the first screen.

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

Tabs work the same way: `ui.Tabs(style, ui.Tab{Title, Icon, Screen}...)`
is `UITabBarController` on iOS (SF Symbol icons) and a bottom bar on
Android; every tab stays mounted; `ui.TabsOf(c).Select(i)` switches from
Go and a user's tap is reported back. `ui.SafeArea` pads clear of the
status bar, home indicator and tab bar.

Modals: `ui.Modals(c).Present(title, screen)` shows a page sheet on iOS
(swipe down to close) and a full-screen layer on Android (back to close);
each has its own stack and title bar, so `Navigation` works inside it.
`ui.Modals(c).Dismiss()` closes it from Go, and a user's close is reported
back.

| Gallery: tap a row → detail | iOS simulator | Android emulator |
|---|---|---|
| Push, tap → committed | Go 2 ms; UIKit push 57–123 ms | 19 ms total |
| Native back → Go follows | ✓ | ✓ |

## Talking to a Nimbus server

```go
// shared/api.go — imported by the Nimbus server and the app
var ListOrders  = api.GET[ListOrdersReq, []Order]("/api/orders")
var CreateOrder = api.POST[NewOrder, Order]("/api/orders")

// in the app
client := api.New("https://orders.example.com")
q := ui.UseQuery(c, "orders", func(ctx context.Context) ([]Order, error) {
	return api.Call(ctx, client, ListOrders, ListOrdersReq{})
})
place := ui.UseAction(c, func(ctx context.Context, o NewOrder) (Order, error) {
	created, err := api.Call(ctx, client, CreateOrder, o)
	form.SetErrors(err) // a Nimbus 422 lands on the form's fields
	return created, err
}, func(Order) { ui.RefetchQuery(c, "orders") })
```

The `api` package depends only on the standard library, so shared code
that imports it builds into the app without pulling in the server. An
endpoint's method, path and request/response types are declared once;
`{name}` path parameters come from fields tagged `path:"name"`, GET/DELETE
requests become query parameters and the rest JSON bodies. A non-2xx
response is an `*api.Error`; a Nimbus 422's field errors come with it.
`ui.UseQuery` fetches off the UI goroutine, caches by key across
components, refreshes stale data in the background and refetches on
demand; `ui.UseAction` runs writes with their progress and result. See
`examples/fullstack` for a Nimbus server and app sharing one contract.

## Theming and dark mode

`ui.Theme` holds an app's tokens (background, surface, text, muted, border,
primary, danger, bar colors, radius, type and spacing scales);
`ui.LightTheme` / `ui.DarkTheme` are built in and `ui.SetTheme(light, dark)`
replaces them. The runtime follows the system's dark mode: `ui.UseTheme(c)`
re-renders with the other theme, unset text, input and spinner colors take
the theme's, and the `Chrome` op restyles what the platform draws —
navigation and tab bars, screen and modal backgrounds, Android's status and
navigation bars.

## Forms

```go
form := ui.UseForm(c, Signup{}, validation.ValidateStruct) // a Nimbus server's own rules
ui.TextField(form.Field("email", func(s *Signup) *string { return &s.Email }),
	ui.FieldOptions{Label: "Email", Keyboard: render.KeyboardEmail})
ui.Pressable(form.Submit(save), style, ui.Text("Sign up", style))
```

`UseForm` takes any `func(T) error`; field errors come from `ui.FieldErrors`,
Nimbus's `validation.ValidationErrors` or anything with
`ToMap() map[string][]string` — so one rule set validates both the server
and the app, without Nimbus Native importing Nimbus. Errors show once a
field is touched, and for every field on submit; submit runs only when the
value is valid. `ui.FieldErrorsOf(err)` reads a server's 422 the same way.

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
