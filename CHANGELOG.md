# Changelog

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
  iPhone; not yet through a store review. Dev mode (reload on save) runs on
  simulators and emulators only.
- No flex-wrap, animations or gestures yet.
- Native plugins answer `unavailable` on the web.
- On the web, each open tab keeps a small UI session on the server.
