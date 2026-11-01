// Package bluetooth talks to Bluetooth LE devices: scales, printers, beacons
// and the like. Characteristics are read and written as bytes.
//
//	nimbus native plugin add bluetooth
//
//	ok, err := bluetooth.Request(ctx)
//	go bluetooth.Scan(scanCtx, func(d bluetooth.Device) { add(d) })
//	err = bluetooth.Connect(ctx, d.ID)
//	b, err := bluetooth.Read(ctx, d.ID, service, characteristic)
//
// Written against the platform APIs, but not yet run: simulators and
// emulators have no Bluetooth.
package bluetooth

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Device is something nearby.
type Device struct {
	ID     string `json:"id"`     // for Connect; it differs between phones
	Name   string `json:"name"`   // its name, if it gives one
	Signal int    `json:"signal"` // RSSI, in dBm
}

// Request asks to use Bluetooth, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "bluetooth", "request", nil)
	return r.Granted, err
}

// Scan calls f with each device it finds until ctx ends. services narrows it
// to those UUIDs. It blocks, so run it from a goroutine in an effect.
//
//nimbus:param f called with each device found
//nimbus:param services service UUIDs to look for; none means any
func Scan(ctx context.Context, f func(Device), services ...string) error {
	if _, err := plugin.CallJSON(ctx, "bluetooth", "scanStart", map[string]any{"services": services}); err != nil {
		return err
	}
	defer call("scanStop", nil)
	seen := map[string]bool{}
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[struct {
			Devices []Device `json:"devices"`
		}](ctx, "bluetooth", "scanRead", nil)
		if err != nil {
			return err
		}
		for _, d := range r.Devices {
			if !seen[d.ID] {
				seen[d.ID] = true
				f(d)
			}
		}
	}
}

func call(method string, args any) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = plugin.CallJSON(ctx, "bluetooth", method, args)
}

// Connect connects to a device, and finds its services.
//
//nimbus:param id the Device's ID
func Connect(ctx context.Context, id string) error {
	_, err := plugin.CallJSON(ctx, "bluetooth", "connect", map[string]string{"id": id})
	return err
}

// Disconnect ends the connection.
//
//nimbus:param id the Device's ID
func Disconnect(ctx context.Context, id string) error {
	_, err := plugin.CallJSON(ctx, "bluetooth", "disconnect", map[string]string{"id": id})
	return err
}

// Read reads a characteristic.
//
//nimbus:param id the Device's ID
//nimbus:param service the service UUID
//nimbus:param characteristic the characteristic UUID
func Read(ctx context.Context, id, service, characteristic string) ([]byte, error) {
	r, err := plugin.Call[struct {
		Value string `json:"value"`
	}](ctx, "bluetooth", "read", map[string]string{"id": id, "service": service, "characteristic": characteristic})
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(r.Value)
}

// Write writes a characteristic, and waits for the device to confirm.
//
//nimbus:param id the Device's ID
//nimbus:param service the service UUID
//nimbus:param characteristic the characteristic UUID
//nimbus:param value the bytes to write
func Write(ctx context.Context, id, service, characteristic string, value []byte) error {
	_, err := plugin.CallJSON(ctx, "bluetooth", "write", map[string]string{"id": id, "service": service, "characteristic": characteristic,
		"value": base64.StdEncoding.EncodeToString(value)})
	return err
}
