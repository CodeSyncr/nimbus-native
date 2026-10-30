package api

import "testing"

func TestServerURLOrder(t *testing.T) {
	t.Setenv("NIMBUS_SERVER_URL", "")
	builtServerURL = ""
	t.Cleanup(func() { builtServerURL = "" })
	if got := ServerURL("http://fallback"); got != "http://fallback" {
		t.Fatalf("fallback: %s", got)
	}
	builtServerURL = "https://built"
	if got := ServerURL("http://fallback"); got != "https://built" {
		t.Fatalf("built: %s", got)
	}
	t.Setenv("NIMBUS_SERVER_URL", "http://dev")
	if got := ServerURL("http://fallback"); got != "http://dev" {
		t.Fatalf("dev: %s", got)
	}
}
