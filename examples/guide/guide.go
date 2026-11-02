// Package guide holds the complete example on each plugin's guide page,
// compiled here so that they can't go stale. tools/docref copies each file
// into its page, as package main.
package guide

import "github.com/CodeSyncr/nimbus-native/ui"

// Screens are the examples, by plugin.
var Screens = map[string]ui.Component{
	"appinfo": AboutApp{}, "appreview": RateUs{}, "appstate": LiveOrders{}, "audio": AskOnce{},
	"backgroundtasks": NightlySync{}, "badge": Unread{}, "barcode": ScanCode{}, "biometrics": Figures{},
	"bluetooth": Nearby{}, "browser": OurStory{}, "calendar": Booking{}, "camera": Review{},
	"clipboard": GiftCode{}, "contacts": SendGift{}, "device": AboutPhone{}, "dialog": CancelOrder{},
	"filepicker": Upload{}, "filesystem": Notes{}, "geofencing": Arrivals{}, "geolocation": WhereAmI{},
	"googlemaps": Delivery{}, "haptics": Feedback{}, "keepawake": Till{}, "keyboard": Note{},
	"linking": CallUs{}, "localnotifications": Remind{}, "motion": Level{}, "network": Online{},
	"nfc": ReadTag{}, "orientation": KitchenScreen{}, "purchases": Membership{}, "push": Notifications{},
	"screenreader": OrderReady{}, "securestore": Session{}, "share": SplitBill{}, "signin": Welcome{},
	"speech": ReadAloud{}, "speechrecognition": VoiceSearch{}, "splashscreen": FirstScreen{}, "statusbar": DarkHero{},
	"barometer": Floors{}, "lightsensor": Ambient{}, "magnetometer": Detector{}, "pedometer": Steps{},
	"brightness": BoardingPass{}, "navigationbar": Kiosk{}, "screencapture": CardNumber{},
	"intentlauncher": FixWifi{}, "mailcomposer": SendReceipt{}, "printing": PrintReceipt{}, "sms": TextFriend{},
	"cellular": PhoneNetwork{}, "imagemanipulator": Thumbnail{}, "localization": Prices{}, "medialibrary": SaveToPhotos{},
	"cameraview":  Viewfinder{},
	"audioplayer": Podcast{}, "audiorecorder": VoiceMemo{},
	"calendarevents":     Agenda{},
	"backgroundlocation": Trip{},
	"oauth":              WorkAccount{},
	"applemaps":          Directions{},
	"receiveshare":       Inbox{},
	"sqlite":             Orders{},
	"symbols":            MenuIcons{},
	"scene":              Showroom{},
	"meshgradient":       Sunset{},
	"storage":            Counter{}, "trackingconsent": Consent{}, "video": Tour{}, "webview": Reservations{},
}
