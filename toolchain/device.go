package toolchain

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// IOSPhone is an iPhone or iPad this Mac can install on, over USB or Wi-Fi
// (paired in Xcode's Devices window), as devicectl lists it.
type IOSPhone struct {
	Name string
	ID   string // CoreDevice identifier, for devicectl
	UDID string // hardware UDID, for xcodebuild
	Wifi bool
}

// IOSPhones lists the paired iPhones and iPads that can be reached now.
func IOSPhones() ([]IOSPhone, error) {
	f, err := os.CreateTemp("", "devices-*.json")
	if err != nil {
		return nil, err
	}
	f.Close()
	defer os.Remove(f.Name())
	if out, err := exec.Command("xcrun", "devicectl", "list", "devices", "--json-output", f.Name()).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("devicectl (Xcode 15 or later): %v\n%s", err, out)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		return nil, err
	}
	var doc struct {
		Result struct {
			Devices []struct {
				Identifier       string `json:"identifier"`
				DeviceProperties struct {
					Name string `json:"name"`
				} `json:"deviceProperties"`
				HardwareProperties struct {
					UDID       string `json:"udid"`
					DeviceType string `json:"deviceType"`
					Platform   string `json:"platform"`
				} `json:"hardwareProperties"`
				ConnectionProperties struct {
					PairingState  string `json:"pairingState"`
					TunnelState   string `json:"tunnelState"`
					TransportType string `json:"transportType"`
				} `json:"connectionProperties"`
			} `json:"devices"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("devicectl's device list: %w", err)
	}
	var phones []IOSPhone
	for _, d := range doc.Result.Devices {
		c, h := d.ConnectionProperties, d.HardwareProperties
		if h.Platform != "iOS" || c.PairingState != "paired" || c.TunnelState == "unavailable" {
			continue
		}
		phones = append(phones, IOSPhone{Name: d.DeviceProperties.Name, ID: d.Identifier, UDID: h.UDID,
			Wifi: c.TransportType == "localNetwork"})
	}
	return phones, nil
}

// PickIOSPhone is the phone matching want (a name or id), or the only one.
func PickIOSPhone(want string) (IOSPhone, error) {
	phones, err := IOSPhones()
	if err != nil {
		return IOSPhone{}, err
	}
	var names []string
	for _, p := range phones {
		if want != "" && (strings.EqualFold(p.Name, want) || p.ID == want || p.UDID == want) {
			return p, nil
		}
		names = append(names, fmt.Sprintf("%q (%s)", p.Name, p.UDID))
	}
	switch {
	case want != "":
		return IOSPhone{}, fmt.Errorf("no reachable iPhone or iPad named %q; reachable: %s", want, strings.Join(names, ", "))
	case len(phones) == 0:
		return IOSPhone{}, fmt.Errorf("no iPhone or iPad reachable: connect one by USB, or pair it for Wi-Fi in Xcode (Window → Devices and Simulators), unlocked and on the same network")
	case len(phones) > 1:
		return IOSPhone{}, fmt.Errorf("several devices reachable; pick one with -udid: %s", strings.Join(names, ", "))
	}
	return phones[0], nil
}

var teamSetting = regexp.MustCompile(`DEVELOPMENT_TEAM = "?([A-Z0-9]{10})"?;`)

// IOSTeam is the Apple team that signs device builds: want (the -team flag
// or NIMBUS_IOS_TEAM), else the Xcode project's own, else the only team
// signed in to Xcode.
func IOSTeam(root, want string) (string, error) {
	if want == "" {
		want = os.Getenv("NIMBUS_IOS_TEAM")
	}
	if want != "" {
		return want, nil
	}
	if b, err := os.ReadFile(filepath.Join(root, "native/ios/Runner.xcodeproj/project.pbxproj")); err == nil {
		if m := teamSetting.FindSubmatch(b); m != nil {
			return string(m[1]), nil
		}
	}
	teams := xcodeTeams()
	switch len(teams) {
	case 0:
		return "", fmt.Errorf("no Apple team to sign with: add your Apple ID in Xcode (Settings → Accounts), then try again")
	case 1:
		for id := range teams {
			return id, nil
		}
	}
	var list []string
	for id, name := range teams {
		list = append(list, fmt.Sprintf("-team %s (%s)", id, name))
	}
	sort.Strings(list)
	return "", fmt.Errorf("several Apple teams in Xcode; pick one (or set NIMBUS_IOS_TEAM, or a team in the Xcode project):\n  %s",
		strings.Join(list, "\n  "))
}

// xcodeTeams are the teams of the accounts signed in to Xcode, by id.
func xcodeTeams() map[string]string {
	out, err := exec.Command("defaults", "read", "com.apple.dt.Xcode", "IDEProvisioningTeams").Output()
	if err != nil {
		return nil
	}
	teams := map[string]string{}
	ids := regexp.MustCompile(`teamID = "?([A-Z0-9]{10})"?;`).FindAllSubmatch(out, -1)
	names := regexp.MustCompile(`teamName = "?([^";]+)"?;`).FindAllSubmatch(out, -1)
	for i, m := range ids {
		name := ""
		if i < len(names) {
			name = string(names[i][1])
		}
		teams[string(m[1])] = name
	}
	return teams
}

// RunIOSPhone installs the app on a phone and launches it, streaming its
// output until it exits or the command is interrupted.
func (e *Env) RunIOSPhone(a *App, appPath string, p IOSPhone) error {
	e.step("install on %s", p.Name)
	if err := e.run(nil, "xcrun", "devicectl", "device", "install", "app", "--device", p.ID, appPath); err != nil {
		return fmt.Errorf("%w\n(is the phone unlocked, with Developer Mode on: Settings → Privacy & Security → Developer Mode?)", err)
	}
	e.step("launch %s", a.ID)
	cmd := exec.Command("xcrun", "devicectl", "device", "process", "launch", "--device", p.ID,
		"--terminate-existing", "--console", a.ID)
	cmd.Stdout, cmd.Stderr = e.Out, e.Out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launch: %w\n(a first run from a new team needs the phone to trust it: Settings → General → VPN & Device Management)", err)
	}
	return nil
}
