package badge

import (
	_ "embed"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

//go:embed web.js
var webJS string

func init() { plugin.RegisterWeb("badge", webJS) }
