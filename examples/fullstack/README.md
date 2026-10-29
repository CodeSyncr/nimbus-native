# Full stack: a Nimbus server and a Nimbus Native app

One Go module, three packages:

| Package | What |
|---|---|
| `shared/` | The contract: `Order`, `NewOrder`, and the endpoints `ListOrders` / `CreateOrder`. Both sides compile against it. |
| `server/` | A Nimbus app serving the endpoints. It owns the validation rules (Nimbus `validation`). |
| `app/` | A Nimbus Native app: lists orders (`ui.UseQuery`), places one from a modal form (`ui.UseAction`), and shows the server's 422 errors on the form (`form.SetErrors`). |

The app imports `shared` and the dependency-free `api` package only — no
server code ends up in the phone binary.

```sh
PORT=3400 go run ./server                                   # terminal 1
nimbus native run ios -name Orders -id space.nimbusgo.orders ./app
adb reverse tcp:3400 tcp:3400                               # Android reaches the Mac's server
nimbus native run android -name Orders -id space.nimbusgo.orders ./app
```

The app's hosts (`app/ios`, `app/android`) include a scripted check that
logs `FULLSTACK RESULT: PASS` after listing, an empty order (server errors on
the form) and a valid order (the sheet closes, the list refreshes).
