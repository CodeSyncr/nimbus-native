package toolchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
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

// XcodeTeams are the teams of the accounts signed in to Xcode, by id.
func XcodeTeams() map[string]string { return xcodeTeams() }

// SaveIOSTeam writes team into the project's Xcode app target, so Xcode
// and later device builds sign with it without asking.
func SaveIOSTeam(root, team string) error {
	p := filepath.Join(root, "native/ios/Runner.xcodeproj/project.pbxproj")
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	s := string(b)
	if teamSetting.MatchString(s) {
		s = teamSetting.ReplaceAllString(s, "DEVELOPMENT_TEAM = "+team+";")
	} else {
		// The app target's settings both carry CODE_SIGN_STYLE.
		s = strings.ReplaceAll(s, "\t\t\t\tCODE_SIGN_STYLE = Automatic;\n",
			"\t\t\t\tCODE_SIGN_STYLE = Automatic;\n\t\t\t\tDEVELOPMENT_TEAM = "+team+";\n")
	}
	return os.WriteFile(p, []byte(s), 0o644)
}

func xcodeTeams() map[string]string {
	// Xcode 16 keeps them by account; older versions in a flat list.
	var out []byte
	for _, key := range []string{"IDEProvisioningTeamByIdentifier", "IDEProvisioningTeams"} {
		if b, err := exec.Command("defaults", "read", "com.apple.dt.Xcode", key).Output(); err == nil {
			out = b
			break
		}
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

// RunIOSPhone installs the app on a phone and opens it (once the phone is
// unlocked), streaming its output until it exits or the command is
// interrupted.
func (e *Env) RunIOSPhone(a *App, appPath string, p IOSPhone) error {
	if err := e.installOnPhone(appPath, p); err != nil {
		return err
	}
	return e.openOnPhone(a.ID, p, true)
}

func (e *Env) installOnPhone(appPath string, p IOSPhone) error {
	e.step("install on %s", p.Name)
	if err := e.run(nil, "xcrun", "devicectl", "device", "install", "app", "--device", p.ID, appPath); err != nil {
		return fmt.Errorf("%w\n(is the phone unlocked, with Developer Mode on: Settings → Privacy & Security → Developer Mode?)", err)
	}
	return nil
}

// openOnPhone launches an installed app. A locked phone takes the install
// but not the launch: it waits (up to 2 minutes) for the phone to be
// unlocked. With console, it then streams the app's output until it exits.
func (e *Env) openOnPhone(id string, p IOSPhone, console bool) error {
	deadline := time.Now().Add(2 * time.Minute)
	for told := false; ; {
		e.step("open %s", id)
		var out bytes.Buffer
		args := []string{"devicectl", "device", "process", "launch", "--device", p.ID, "--terminate-existing"}
		if console {
			args = append(args, "--console")
		}
		cmd := exec.Command("xcrun", append(args, id)...)
		cmd.Stdout, cmd.Stderr = io.MultiWriter(e.Out, &out), io.MultiWriter(e.Out, &out)
		if !console {
			cmd.Stdout, cmd.Stderr = &out, &out
		}
		err := cmd.Run()
		if err == nil || strings.Contains(out.String(), "Launched application") {
			return nil // it ran (and, with console, has ended)
		}
		if locked(out.String()) && time.Now().Before(deadline) {
			if !told {
				Warnf(e.Out, "Unlock %s to open the app (waiting up to 2 minutes)…", p.Name)
				told = true
			}
			time.Sleep(2 * time.Second)
			continue
		}
		hint := "is the phone unlocked?"
		if strings.Contains(strings.ToLower(out.String()), "trust") || strings.Contains(out.String(), "profile") {
			hint = "the phone must trust this developer: Settings → General → VPN & Device Management"
		}
		return fmt.Errorf("opening the app: %w (%s)\n%s", err, hint, out.String())
	}
}

// locked reports whether devicectl failed because the phone is locked.
func locked(out string) bool {
	out = strings.ToLower(out)
	return strings.Contains(out, "locked") || strings.Contains(out, "unlock")
}
