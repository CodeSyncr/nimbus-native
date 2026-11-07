package googlemaps

import (
	_ "embed"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

//go:embed web.js
var webJS string

func init() { plugin.RegisterWeb("googlemaps", webJS) }
