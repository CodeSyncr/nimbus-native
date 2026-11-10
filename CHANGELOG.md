# Changelog

## Unreleased

- Fixed: on the web, scroll views didn't scroll. The browser does the
  layout there, so their `Scroll` op was never sent, and with it went
  `overflow-y: auto`; a page longer than the window was cut off.
- `Button.Copies` and `Pressable(...).Copies(text)`: a tap copies the text,
  then the handler runs. Browsers copy it inside the tap itself, which
  Safari requires (it refuses `clipboard.Set` from a handler); phones use
  the clipboard plugin.
- Plugins in the browser. On the web, a plugin call goes from the tab's
  session on the server to the visitor's browser, which answers with its
  own APIs: clipboard, share, geolocation, device, network, app state, keep
  awake, browser, localization, speech, badge, printing, mail composer, SMS,
  file picker, camera (through the file chooser), audio player, audio
  recorder, motion, local notifications, storage and secure store. App code
  doesn't change; the call finds its tab through the ctx that `UseQuery`,
  `UseAction` and effects hand out (`plugin.WithBrowser`,
  `plugin.InBrowser`). A plugin gives itself a browser side with
  `plugin.RegisterWeb` and a `web.js` that calls `NN.plugin`; the page loads
  it the first time it's needed.
- More plugins in the browser: dialogs, toasts and date pickers drawn in the
  page, linking, app info, keyboard, screen reader announcements, calendar
  (an .ics file), saving media (a download), contact picking, NFC, speech
  recognition, barcodes (the camera with BarcodeDetector), orientation,
  tracking consent and the status bar's color.
- OAuth and Google and Apple sign-in on the web, in a popup that comes back
  to the site's `/_nn/oauth`. `signin.Options.AppleServicesID` is Apple's
  Services ID for the web.
- Native views on the web: a plugin draws its kind with `NN.view`. Video
  plays in a `<video>` and the web view is an iframe.
- `DateField` opens the browser's date picker on the web.
- The last plugins in the browser: Google Maps (Maps JavaScript API, with
  `googlemaps.WebKey`), Apple Maps (MapKit JS, with `applemaps.WebToken`;
  Fallback without one), 3D scenes in WebGL 2 with shadows, orbit and taps,
  mesh gradients, symbols from the Material font, voice turns through the
  browser's microphone, and haptics through `haptics.For(ctx)`.
- `ui.Ctx.Context` is the context for plugin calls from an event handler;
  on the web it leads to the visitor's tab.
- On the web, one-line text ends in an ellipsis instead of breaking a word,
  and font files with spaces in their names load.
- Effects, queries and actions in a web session get a context that ends when
  the tab's session closes.
- Plugins' clean-up calls (stopping a sensor, a watch or speech) reach the
  same phone or tab as the call they end.

## 0.0.13 (2026-11-07)

The biggest release of the beta: thirty components, a plugin system with
over seventy plugins, and a drawing layer.

**Screens and look**

- Thirty components drawn in Go from the theme (buttons, inputs, lists,
  sheets, toasts, menus, tooltips, carousels and more), passed straight in as
  children; `Render` returns a `ui.Component`.
- Drags, swipes and pinches on any view; `OnScroll`; `OnSubmit`,
  `AutoFocus` and `StickToEnd` for inputs and chats.
- `Style`: blur and Liquid Glass (`BlurGlass`, iOS 26), gradients, shadows,
  per-corner radius, `Scale` and `Rotate`, letter spacing, line height,
  tabular digits and shrink to fit. `ui.Rich` for text in runs.
- Android Back closes an open overlay first; Select works in a browser.

**Canvas**

- `ui.Canvas`, for shapes Go draws: paths and curves, arcs, ovals, SVG path data (`Painter.Path`),
  even-odd fills, clipping, `Save`/`Restore`, translate, rotate and scale,
  layers with opacity, blend modes, shadows, dashes, line caps and joins,
  sweep gradients, gradient strokes, pictures and wrapped text. Each phone
  draws it with its own graphics; nothing is added to the app.
- Canvas animations the phone runs itself, frame by frame, without a render
  from Go: `ui.UseAnimation` (timed, eased or springy, repeating,
  reversing, `OnDone`, `Play`), `ui.UseTween` (a number that glides to a new
  target), `Painter.Motion` to move, turn, scale and fade, and
  `Painter.Trim` to draw part of a stroke.
- Canvas touch: `Painter.Tag` names shapes, `OnTap` reports the topmost
  tagged shape under a tap (where the phone drew it, animations included),
  and `ui.UseDrag` is a number the finger moves on the phone, along one
  axis, from a handle shape or anywhere; Go hears where it was let go and
  can snap it. A drag across leaves vertical scrolling to the page, and the
  other way round.
- Canvas shaders: `ui.NewShader` takes a shader written once, in AGSL, and
  `Painter.FillShader` fills a path with it, its uniforms animating like
  any number. Go turns it into each phone's language: AGSL on Android 13+,
  Metal on iOS, WebGL 2 on the web; older Android fills with a fallback
  color.

**Plugins**

- `nimbus native plugin add|remove|list`; plugins are Go packages with Swift
  and Kotlin handlers, and third-party modules install with
  `plugin add <module>[@version]`.
- Device and system: device, battery, network, cellular, keyboard, status
  bar, navigation bar, splash screen, brightness, keep awake, orientation,
  app state, badge, screen reader, localization, haptics, hinge and split
  view.
- Sensors and place: location, background location, geofencing, motion,
  barometer, magnetometer, light sensor, pedometer.
- Media: camera, camera view, file picker, media library, image
  manipulator, audio, audio player and recorder, video, speech, speech
  recognition, barcode, screen capture, Live Photos (iOS).
- Data and sharing: SQLite (`database/sql`), filesystem, share, receiving
  shares, clipboard, contacts, calendar and calendar events, mail, SMS,
  printing, intent launcher, browser, web view.
- Accounts and services: push, Google Maps, Apple Maps, OAuth, sign-in,
  in-app purchases, tracking consent, app review, biometrics, local
  notifications, background tasks, Bluetooth, NFC.
- Graphics: 3D scenes (Metal and Vulkan, glTF models, shadows, an orbit
  camera), mesh gradients and the platforms' own symbols.
- `ui.Native` places a plugin's native view inside a screen.

**Tooling**

- The nimbus look for `dev`, `run` and `build`; `-server` builds the app's
  server address into it.
- `nimbus native plugin add <module>[@version]` installs third-party Go module
  plugins, stages their Swift and Kotlin handlers in the project, and applies
  their declared native dependencies and permissions. Plugin documentation in
  nimbus-starter covers publishing and using modules.
- iOS calls the app by its name, not "Runner", where it doesn't use the
  display name, such as "… wants to use microsoftonline.com to sign in". The
  bundle, executable and Swift module are still Runner. `nimbus native
  upgrade` (or `sync`) updates projects made before.
- The guide's API reference can describe parameters in pairs (`x, y`).

**Fixes**

- `UseQuery` keeps the hooks after it in place.
- A growing Scroll fills its space instead of growing to its content.
- `Style.Hidden` hides the native view; Text keeps its padding on iOS and
  Android.
- A dev shell left open by another session can't take over.
- The booted simulator is found by device type, not name.
- `plugin add` checks every plugin before changing the project.

## 0.0.12 (2026-10-30)

- Projects with a Nimbus server are whole Nimbus apps: `nimbus native new`
  (nimbus v1.16.1+) makes the app with `nimbus new`'s questions and files
  (`.env`, `config/`, `start/`, controllers, migrations) and adds the
  native app; `start/native.go` serves the app's pages after your own
  routes. `nimbus native dev` runs that server with its `.env`.
- Or the phone app alone: `nimbus native new -server none`, for a backend
  of your own (Node, Go, anything over HTTP and JSON). This is also what
  `nimbus-native new` makes when run directly.
- Servers build with `CGO_ENABLED=0`: the native bridge is left out of
  builds without cgo.
- `nimbus native font add` also copies fonts to `public/fonts`, where the
  web app serves them from.

## 0.0.11 (2026-10-30)

- `nimbus native dev ios|android` also runs the project's Nimbus server and
  web app, restarted on every save: no separate `go run .`. It takes the
  next free port when 3333 is busy, `o` opens the web app, and
  `-no-server` leaves it out. On Android the port is reversed through adb.
- New projects ignore the server's `database.sqlite`.
- Boolean flags (`-sim`, `-release`, `-no-server`) no longer swallow the
  argument after them.

## 0.0.10 (2026-10-30)

- A dev console: `nimbus native dev` says where the app runs, colors panics
  and errors, reports an app process that stops, and takes single keys:
  `r` reload, `R` restart (rebuild and reinstall), `c` clear, `q` quit,
  `h` help.
- `nimbus native upgrade [version]` moves a project to the latest (or a
  named) release: `go.mod`, then the release's own `nimbus native sync`
  updates the kits under `native/` and the generated Swift.
- `nimbus native dev` checks its platform argument before doing anything.

## 0.0.9 (2026-10-30)

- Flex-wrap: `Style{Wrap: true}` breaks rows (and columns) into lines,
  `LineGap` apart; checked against Chromium.
- Links into the app: every project has a URL scheme, its module name;
  `myapp://orders/42` opens `/orders/42`, pushed on the visible stack or as a
  sheet, at launch or while running. `ui.HandleURL` takes over.
- `ui.Slider`, `Pressable.OnLongPress`, and `Style.Transition` (animated
  changes of position, opacity and background).
- Fonts: `nimbus native font add <font.ttf>` and `Style.FontFamily`, on iOS,
  Android and the web.

## 0.0.8 (2026-10-30)

- Store builds: `nimbus native build ios -release` archives and exports an
  `.ipa` (App Store Connect, ad-hoc or development); `nimbus native build
  android -release` makes the Play Store `.aab` and a release `.apk`.
  `-version` and `-build` set the version on both.
- `nimbus native keystore` makes the Android release key and
  `keystore.properties`, kept out of git; release builds use it.
- `nimbus native icon <image>`: the iOS App Store icon and Android adaptive
  icons from one picture, plus the launch screen color on both (Android 12's
  splash included). New projects start with an icon and launch screen in the
  app's colors.
- The shop template is gone: new projects are the counter.

## 0.0.7 (2026-10-30)

- `nimbus native dev` runs on a connected phone too, chosen like `run`
  (`-sim`, `-udid`, `-team`). An iPhone reaches dev mode over Wi-Fi: the
  relay listens on the Mac's network address, and the dev build asks once
  for local network access. Checked on an iPhone 14 Pro: a save is on the
  phone in about 3 seconds.

## 0.0.6 (2026-10-30)

- `nimbus native run ios|android` runs on a connected phone by default,
  and on the simulator or emulator when there is none. With several
  devices it asks which one; `-sim` forces the simulator or emulator, and
  `-udid` picks a device.
- The first iPhone run asks which Apple team signs the app (when Xcode has
  several) and saves it in the Xcode project.
- Android runs, dev mode included, address the chosen device, so a phone
  and an emulator can both be connected.
- Reads the teams of Xcode 16, which keeps them under a new key.

## 0.0.5 (2026-10-30)

- `nimbus native run ios -device` opens the app after installing it even
  when the phone is locked: it asks you to unlock the phone and opens the
  app as soon as you do (it waits up to 2 minutes).

## 0.0.4 (2026-10-30)

- `nimbus native run ios -device`: builds a project for a real iPhone or
  iPad, signs it automatically with your team (`-team`, `NIMBUS_IOS_TEAM`
  or the Xcode project's), registers the phone, installs it over USB or
  Wi-Fi and launches it with its output streaming. Checked on an iPhone 14
  Pro over Wi-Fi.

## 0.0.3 (2026-10-30)

- `nimbus native new` makes a counter: one polished screen with a goal
  and progress bar, step sizes, the count saved between launches, and
  light and dark themes, on iOS, Android and the web. The shop is still
  there: `nimbus native new -template shop <dir>`.
- Outside a project, `nimbus native dev|build|run|gen` say so and how to
  make one, instead of failing in `go list`.
- Generated apps install their theme in the shared `app` package, so the
  web shows it too (it used to show the default theme).

## 0.0.2 (2026-10-30)

- Fixes iOS projects made from the published module: the Swift package's
  C header was a symlink, which Go module zips leave out. v0.0.1 is
  retracted.
- `nimbus native` ships in the Nimbus CLI from v1.16.0:
  `go install github.com/CodeSyncr/nimbus/cmd/nimbus@latest`.

## 0.0.1: first beta (2026-10-30)

Native iOS and Android apps in Go, and the same app on the web.

**Engine**
- Own toolchain (no gomobile): Go as a static library on iOS and a shared
  library on Android, behind a small C ABI; typed wire messages generated
  for Go, Swift and Kotlin.
- A flexbox engine in Go, checked against Chromium on 62 fixtures.
- Components and hooks (`State`, `Reducer`, `Effect`, `Memo`, `Ref`,
  `UseQuery`, `UseAction`, `UseForm`, `UseTheme`), a keyed reconciler, and
  `ui.Store` for app-wide state.
- Native views: text, images, pressables, scroll views, virtualized lists
  with pull to refresh, text inputs (single and multiline), switches,
  spinners; stacks, tabs and sheets with the platforms' own transitions.
- Safe areas and keyboard avoidance; light and dark themes that follow
  the system.

**Projects**
- `nimbus native new` makes a feature-based Go module with a Nimbus
  server, `native/ios` (an Xcode project) and `native/android` (a Gradle
  project). The sample is a small shop: catalog, cart, sign-in, checkout,
  orders.
- Folder routing: `pages/orders/_id/page.go` is `/orders/:id`, generated
  into a static table; `router.Push` and `router.Link`.
- `nimbus native dev`: the app runs on your machine and reloads on save,
  keeping the screen you were on. Works in projects and single directories.
- The phone build refuses server code (Nimbus, `server` packages).

**Server and web**
- Typed endpoints (`api.GET[Req, Resp]`) shared by the server and the app;
  Nimbus validation errors land on form fields.
- The same app on the web, served by Nimbus: server-rendered HTML with the
  page's data, title, description and Open Graph tags, a sitemap and
  robots.txt, then live updates over Server-Sent Events (an 8 KB script).

**Native plugins**
- `plugin.Call` to Swift or Kotlin with JSON arguments; register your own.
- Built in: secure storage (Keychain, Android Keystore), key-value
  storage, app info, opening links, and dialogs (alerts, confirmations,
  action sheets, toasts, date and time pickers).

**Known limitations**
- Tested on simulators, emulators, desktop and mobile browsers, and an
  iPhone; not yet through a store review.
- No flex-wrap, animations or gestures yet.
- Native plugins answer `unavailable` on the web.
- On the web, each open tab keeps a small UI session on the server.
