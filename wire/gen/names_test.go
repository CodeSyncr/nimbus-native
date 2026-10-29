package gen

import "testing"

func TestNames(t *testing.T) {
	for in, want := range map[string]string{"ID": "id", "UserID": "userID", "HTTPServer": "httpServer", "Name": "name", "X": "x", "already": "already"} {
		if got := lowerCamel(in); got != want {
			t.Errorf("lowerCamel(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"BigText": "BIG_TEXT", "Text": "TEXT", "HTTPCode": "HTTP_CODE", "A1": "A1"} {
		if got := upperSnake(in); got != want {
			t.Errorf("upperSnake(%q) = %q, want %q", in, got, want)
		}
	}
	if swiftName("Default") != "`default`" || kotlinName("Default") != "default" || kotlinName("When") != "`when`" {
		t.Error("keywords not escaped")
	}
}
