package toolchain

import (
	"strings"
	"testing"
)

func TestNativeArgs(t *testing.T) {
	got := strings.Join(nativeArgs("/r/plugins/scene", nativeLib{Name: "nimbusscene", Sources: []string{"android/scene_vk.c"}, LDFlags: []string{"-lvulkan"}}, "/o/libnimbusscene.so"), " ")
	want := "-shared -fPIC -O2 -Wall -I/r/plugins/scene -o /o/libnimbusscene.so /r/plugins/scene/android/scene_vk.c -lvulkan -Wl,-z,max-page-size=16384"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestNoNativeLibs(t *testing.T) {
	e := &Env{Repo: ".."}
	got, err := e.pluginNativeLibs(t.TempDir(), AndroidABIs["arm64-v8a"], t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}
