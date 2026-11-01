package main

import "github.com/CodeSyncr/nimbus-native/ui"

// shot is a plugin at work in the cafe's app: a sample screen for its guide
// page. The values are made up, so the pictures read the same on both phones.
type shot struct {
	title, line string
	icon        string
	lead, text  string
	facts       [][2]string
	rows        []ui.ListRow
	button      string
	quiet       string // a second, quieter button
}

func init() {
	for name, s := range shots {
		screens[name] = func() ui.Component { return s }
	}
}

func (s shot) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	var kids []ui.Component
	if s.lead != "" {
		kids = append(kids, ui.View(ui.Style{Radius: 22, Padding: ui.All(22), Gap: 6, Gradient: ui.LinearGradient(ui.Point{X: 0, Y: 0}, ui.Point{X: 1, Y: 1}, ui.Hex("#0d7377"), ui.Hex("#14a098"))},
			ui.View(ui.Style{Width: ui.Pt(52), Height: ui.Pt(52), Radius: 16, Background: ui.Hex("#ffffff30"), AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter, Margin: ui.Edges{Bottom: 10}},
				ui.Icon(s.icon, 28, ui.Hex("#ffffff"))),
			ui.Text(s.lead, ui.Style{FontSize: 22, FontWeight: 700, LetterSpacing: -0.3, Color: ui.Hex("#ffffff")}),
			ui.Text(s.text, ui.Style{FontSize: 15, Color: ui.Hex("#ffffffd8")})))
	}
	if len(s.facts) > 0 {
		var lines []ui.Component
		for i, f := range s.facts {
			if i > 0 {
				lines = append(lines, ui.Divider{})
			}
			lines = append(lines, ui.View(ui.Style{Direction: ui.Row, Justify: ui.JustifySpaceBetween, Padding: ui.XY(16, 13)},
				ui.Text(f[0], ui.Style{FontSize: 15, Color: th.TextMuted}),
				ui.Text(f[1], ui.Style{FontSize: 15, FontWeight: 600, Color: th.Text})))
		}
		kids = append(kids, ui.View(ui.Style{Radius: 16, Background: th.Surface, BorderWidth: 1, BorderColor: th.Border}, lines...))
	}
	if len(s.rows) > 0 {
		var lines []ui.Component
		for _, r := range s.rows {
			lines = append(lines, r)
		}
		kids = append(kids, ui.View(ui.Style{Radius: 16, Background: th.Surface, BorderWidth: 1, BorderColor: th.Border}, lines...))
	}
	nop := func() {}
	if s.button != "" {
		kids = append(kids, ui.Button{Label: s.button, Full: true, OnPress: nop})
	}
	if s.quiet != "" {
		kids = append(kids, ui.Button{Label: s.quiet, Full: true, Variant: ui.Tonal, OnPress: nop})
	}
	return page(c, s.title, s.line, kids...)
}

var shots = map[string]shot{
	"haptics": {title: "Your order", line: "Every tap answers back.", icon: "check", lead: "Order placed", text: "A firm double tap says it went through.",
		facts: [][2]string{{"Add to cart", "Light tap"}, {"Order placed", "Success"}, {"Card declined", "Error"}}, button: "Place order"},
	"audio": {title: "Ask Vani", line: "Say what you'd like.", icon: "mic", lead: "Listening…", text: "“One flat white and a banana bread, please.”",
		facts: [][2]string{{"Heard", "4.2 seconds"}, {"Noise", "Suppressed"}}, button: "Stop", quiet: "Type instead"},
	"device": {title: "About this phone", line: "Sent along with a support ticket.",
		facts: [][2]string{{"Model", "Pixel 9"}, {"System", "Android 15"}, {"Memory", "8 GB"}, {"Battery", "82%"}, {"Language", "en-IN"}}, button: "Contact support"},
	"network": {title: "Orders", line: "We'll send it when you're back.", icon: "wifi", lead: "You're offline", text: "2 orders are waiting to go out.",
		facts: [][2]string{{"Connection", "None"}, {"Last synced", "4 minutes ago"}}, button: "Try again"},
	"filesystem": {title: "Receipts", line: "Kept on this phone.",
		rows:   []ui.ListRow{{Title: "receipt-1042.pdf", Subtitle: "Today, 42 KB", Icon: "file"}, {Title: "receipt-1041.pdf", Subtitle: "Yesterday, 39 KB", Icon: "file"}, {Title: "menu-october.pdf", Subtitle: "Oct 1, 1.2 MB", Icon: "folder"}},
		button: "Export all"},
	"keyboard": {title: "A note", line: "Tap Done and it tucks away.",
		facts: [][2]string{{"Table", "7"}, {"Note", "No sugar, extra hot"}}, button: "Done"},
	"status-bar": {title: "Roastery", line: "The clock's strip follows the theme.", icon: "sun", lead: "Light content", text: "White icons over a dark photo, and back again when it scrolls away.",
		facts: [][2]string{{"Style", "Light"}, {"Hidden", "No"}}},
	"splash-screen": {title: "Monsoon", line: "Ready before it shows.", icon: "clock", lead: "Loading your menu", text: "The launch color stays up until this first screen is done.",
		facts: [][2]string{{"First screen", "Ready"}, {"Time to show", "310 ms"}}},
	"share": {title: "Order 1042", line: "Send it to whoever's paying.", icon: "share", lead: "Split the bill", text: "Asha owes you ₹580 for lunch.",
		facts: [][2]string{{"Total", "₹1,160"}, {"Your share", "₹580"}}, button: "Share with Asha"},
	"clipboard": {title: "Your gift", line: "Tap to copy the code.", icon: "gift", lead: "MONSOON-25", text: "25% off your next order.",
		button: "Copy code", quiet: "Paste a code"},
	"screen-reader": {title: "Your order", line: "Said out loud when it changes.", icon: "eye", lead: "“Your order is ready”", text: "VoiceOver and TalkBack announce it by themselves.",
		facts: [][2]string{{"Screen reader", "On"}, {"Last said", "Ready for pickup"}}},
	"app-badge": {title: "Inbox", line: "The number on the app's icon.", icon: "bell", lead: "3 unread", text: "The icon says so before they open the app.",
		rows: []ui.ListRow{{Title: "Your flat white is ready", Subtitle: "2 min ago", Icon: "bag"}, {Title: "New: cold brew", Subtitle: "Today", Icon: "star"}}, button: "Mark all as read"},
	"geolocation": {title: "Nearest cafe", line: "Based on where you are.", icon: "map-pin", lead: "Koregaon Park", text: "650 m away, an 8 minute walk.",
		facts: [][2]string{{"Latitude", "18.5362"}, {"Longitude", "73.8930"}, {"Accuracy", "12 m"}}, button: "Start walking"},
	"geofencing": {title: "Order ahead", line: "We notice when you're close.", icon: "map-pin", lead: "You've arrived", text: "We've started your flat white.",
		facts: [][2]string{{"Place", "Koregaon Park"}, {"Radius", "150 m"}, {"Entered", "9:38"}}},
	"motion": {title: "Shake to tip", line: "From the accelerometer.", icon: "refresh", lead: "Shake your phone", text: "A good shake rounds up your bill for the team.",
		facts: [][2]string{{"x", "0.02 g"}, {"y", "-0.11 g"}, {"z", "-0.98 g"}}},
	"camera": {title: "Your review", line: "Add a photo of your order.", icon: "camera", lead: "Show us your latte art", text: "The best one each week wins a free coffee.",
		button: "Take a photo", quiet: "Choose from library"},
	"file-picker": {title: "Catering", line: "Send us your event details.",
		rows:   []ui.ListRow{{Title: "guest-list.pdf", Subtitle: "1.1 MB, ready to send", Icon: "file"}, {Title: "venue-map.png", Subtitle: "640 KB, ready to send", Icon: "image"}},
		button: "Choose a file"},
	"biometrics": {title: "Today's figures", line: "Only for the manager.", icon: "lock", lead: "Confirm it's you", text: "Face ID or a fingerprint, every time this opens.",
		button: "Unlock"},
	"local-notifications": {title: "Reminders", line: "Set on the phone, no server needed.",
		rows:   []ui.ListRow{{Title: "Your loyalty stamp expires", Subtitle: "Tomorrow, 10:00", Icon: "bell"}, {Title: "Order your usual", Subtitle: "Weekdays, 8:30", Icon: "clock"}},
		button: "Add a reminder"},
	"browser": {title: "Our story", line: "Opens inside the app.",
		rows:   []ui.ListRow{{Title: "Where our beans come from", Subtitle: "monsooncoffee.in", Icon: "globe"}, {Title: "Privacy policy", Subtitle: "monsooncoffee.in", Icon: "lock"}},
		button: "Read more"},
	"webview": {title: "Reservations", line: "A page inside your screen.", icon: "globe", lead: "book.monsooncoffee.in", text: "Your booking partner's page, between your own header and footer.",
		button: "Book for 2 at 7:30"},
	"contacts": {title: "Send a gift", line: "Choose who it's for.",
		rows:   []ui.ListRow{{Title: "Asha Rao", Subtitle: "+91 98200 11223", Icon: "user"}, {Title: "Dev Mehta", Subtitle: "+91 98765 43210", Icon: "user"}, {Title: "Sana Khan", Subtitle: "+91 99000 12345", Icon: "user"}},
		button: "Choose from contacts"},
	"calendar": {title: "Your booking", line: "Add it, and it won't slip your mind.", icon: "calendar", lead: "Saturday, 7:30 pm", text: "Table for 2 at Koregaon Park.",
		button: "Add to calendar"},
	"app-state": {title: "Live orders", line: "Pauses when the app is away.", icon: "refresh", lead: "Back in the app", text: "Refreshed the moment you returned.",
		facts: [][2]string{{"State", "Active"}, {"Away for", "2 minutes"}, {"New orders", "3"}}},
	"keep-awake": {title: "Till", line: "The screen stays on all shift.", icon: "sun", lead: "Staying awake", text: "Order 1042 is on the screen, and it won't dim.",
		facts: [][2]string{{"Screen", "Stays on"}, {"Till", "Open"}}, quiet: "Let it sleep"},
	"orientation": {title: "Kitchen screen", line: "Locked sideways on the stand.", icon: "refresh", lead: "Landscape only", text: "The display stays put when the tablet is bumped.",
		facts: [][2]string{{"Locked to", "Landscape"}}, quiet: "Unlock"},
	"app-review": {title: "Thanks!", line: "Your tenth order.", icon: "star", lead: "Enjoying Monsoon?", text: "The system's own prompt asks, right when they're happy.",
		button: "Rate the app", quiet: "Not now"},
	"speech": {title: "Today's recipe", line: "Read aloud, hands free.", icon: "play", lead: "Step 3 of 5", text: "“Pour slowly in circles for thirty seconds.”",
		facts: [][2]string{{"Voice", "English (India)"}, {"Rate", "Normal"}}, button: "Read next step"},
	"video": {title: "How we roast", line: "Plays inside your screen.", icon: "play", lead: "From bean to cup", text: "2 minutes 14 seconds, with the system's own controls.",
		facts: [][2]string{{"Status", "Ready"}, {"Length", "2:14"}}, button: "Play"},
	"speech-recognition": {title: "Search the menu", line: "Say what you're after.", icon: "mic", lead: "“Something cold”", text: "Written out as they speak.",
		rows: []ui.ListRow{{Title: "Cold brew", Subtitle: "₹260", Icon: "tag"}, {Title: "Iced latte", Subtitle: "₹240", Icon: "tag"}}},
	"background-tasks": {title: "Sync", line: "Runs with the app closed.", icon: "refresh", lead: "Menu is up to date", text: "Fetched overnight, without opening the app.",
		facts: [][2]string{{"Task", "sync-menu"}, {"Last run", "3:12 am"}, {"Next", "In about 6 hours"}}},
	"bluetooth": {title: "Printers", line: "Nearby Bluetooth devices.",
		rows:   []ui.ListRow{{Title: "Receipt printer", Subtitle: "Connected", Icon: "link"}, {Title: "Kitchen printer", Subtitle: "-64 dBm", Icon: "wifi"}, {Title: "Coffee scale", Subtitle: "-71 dBm", Icon: "wifi"}},
		button: "Scan again"},
	"nfc": {title: "Loyalty", line: "Hold your phone to the tag.", icon: "tag", lead: "Stamp collected", text: "Tag on the counter at Koregaon Park.",
		facts: [][2]string{{"Tag", "04:A2:7F:1C"}, {"Stamps", "7 of 10"}}, button: "Scan a tag"},
	"push": {title: "Notifications", line: "From your server, to any phone.", icon: "bell", lead: "Your order is ready", text: "Pick it up at the counter.",
		facts: [][2]string{{"Permission", "Allowed"}, {"Sent by", "Your server"}}, button: "Send a test"},
	"google-maps": {title: "Your delivery", line: "Follow it on the map.", icon: "map-pin", lead: "Rider is 4 minutes away", text: "Pins, routes and camera moves in your own screen.",
		facts: [][2]string{{"Rider", "Imran"}, {"Distance", "1.2 km"}, {"Arrives", "9:45"}}, button: "Call rider"},
	"barcode": {title: "Scan & pay", line: "Point at the code on the table.", icon: "search", lead: "Table 7", text: "QR code found: your bill is ready.",
		facts: [][2]string{{"Bill", "₹580"}, {"Format", "QR"}}, button: "Pay now", quiet: "Scan another"},
	"app-info": {title: "About", line: "For when they ask for help.", icon: "info", lead: "Version 2.4.0", text: "Build 118, on iOS and Android alike.",
		facts: [][2]string{{"App ID", "in.monsoon.app"}, {"Language", "en-IN"}}},
	"dialog": {title: "Order 1042", line: "The platform's own alert, in Go.", icon: "alert", lead: "Cancel this order?", text: "It can't be undone.",
		button: "Keep it", quiet: "Cancel order"},
	"secure-store": {title: "Your session", line: "Encrypted on this phone.", icon: "lock", lead: "Signed in", text: "Your token is kept in the Keychain or Keystore.",
		facts: [][2]string{{"Key", "session"}, {"Stored", "Encrypted"}}, quiet: "Sign out"},
	"storage": {title: "Settings", line: "Still here after a restart.", icon: "settings", lead: "Saved", text: "Small things the app should remember.",
		facts: [][2]string{{"theme", "dark"}, {"visits", "12"}, {"last-tab", "orders"}}},
	"linking": {title: "Contact us", line: "Opens the right app.", icon: "phone", lead: "Monsoon Coffee", text: "Koregaon Park, open until 10.",
		rows: []ui.ListRow{{Title: "Call the cafe", Subtitle: "+91 20 1234 5678", Icon: "phone"}, {Title: "Email us", Subtitle: "hello@monsoon.example", Icon: "mail"}}, button: "Get directions"},
	"barometer": {title: "Hill climb", line: "How far up you've come.", icon: "arrow-up", lead: "42 metres climbed", text: "Worked out from the air pressure.",
		facts: [][2]string{{"Pressure", "1008.4 hPa"}, {"Floors", "About 14"}}},
	"magnetometer": {title: "Find the safe", line: "It reads the field around the phone.", icon: "search", lead: "Something's close", text: "The field is far stronger than the Earth's.",
		facts: [][2]string{{"Strength", "212 µT"}, {"x", "-31.2"}, {"y", "18.6"}, {"z", "208.9"}}},
	"light-sensor": {title: "Reading mode", line: "It follows the room.", icon: "moon", lead: "It's dim in here", text: "We've turned on the dark menu.",
		facts: [][2]string{{"Light", "8 lux"}, {"Dark menu", "On"}}},
	"pedometer": {title: "Walk for coffee", line: "Every step earns a stamp.", icon: "user", lead: "6,240 steps", text: "3,760 more for a free flat white.",
		facts: [][2]string{{"Today", "6,240"}, {"Goal", "10,000"}}, button: "Start a walk"},
	"brightness": {title: "Boarding pass", line: "Turned up so the scanner can read it.", icon: "sun", lead: "Gate 12, seat 4A", text: "The screen goes back to normal when you leave.",
		facts: [][2]string{{"Brightness", "100%"}, {"Boards", "9:50"}}},
	"navigation-bar": {title: "Full screen", line: "Android's bar, matched or out of the way.", icon: "menu", lead: "Till mode", text: "The bar hides, and a swipe up brings it back.",
		facts: [][2]string{{"Bar", "Hidden"}, {"Buttons", "Light"}}, button: "Show the bar"},
	"screen-capture": {title: "Your card", line: "Kept out of screenshots.", icon: "card", lead: "•••• 4242", text: "Protected while it's on screen.",
		facts: [][2]string{{"Android", "Screenshots blocked"}, {"iOS", "Covered in recordings"}}},
	"mail-composer": {title: "Your receipt", line: "They check it, then send it.", icon: "mail", lead: "Email a copy", text: "It opens in their own mail app, filled in.",
		facts: [][2]string{{"To", "accounts@example.com"}, {"Attached", "receipt.pdf"}}, button: "Email my receipt"},
	"sms": {title: "Tell a friend", line: "In their own messages app.", icon: "send", lead: "Table for two at 8?", text: "Filled in, for them to check and send.",
		facts: [][2]string{{"To", "+91 20 1234 5678"}}, button: "Text a friend"},
	"printing": {title: "Order 1042", line: "HTML or a PDF, on any printer.", icon: "file", lead: "Ready to print", text: "The system's dialog shows a preview first.",
		facts: [][2]string{{"Pages", "1"}, {"Paper", "A4"}}, button: "Print the receipt"},
	"intent-launcher": {title: "Can't connect?", line: "Android's own settings, one tap away.", icon: "wifi", lead: "Check your Wi-Fi", text: "We'll carry on when you come back.",
		rows: []ui.ListRow{{Title: "Wi-Fi settings", Icon: "wifi"}, {Title: "Notification settings", Icon: "bell"}, {Title: "This app's settings", Icon: "settings"}}},
	"purchases": {title: "Monsoon Club", line: "Free coffee, every Friday.", icon: "star", lead: "₹499 a month", text: "Cancel any time from the store.",
		facts: [][2]string{{"Plan", "Monthly"}, {"Renews", "Dec 1"}}, button: "Join for ₹499", quiet: "Restore purchases"},
	"sign-in": {title: "Welcome back", line: "One tap, no passwords.", icon: "user", lead: "Keep your stamps", text: "Sign in and they follow you to any phone.",
		button: "Continue with Apple", quiet: "Continue with Google"},
	"tracking-consent": {title: "Your choice", line: "We ask before anything else.", icon: "eye", lead: "Better offers?", text: "Allow tracking and we'll show offers you might like. Say no and everything still works.",
		facts: [][2]string{{"Status", "Not asked"}}, button: "Continue"},
}
