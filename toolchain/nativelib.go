package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
)

// nativeLib is a C library a plugin brings for Android: its sources, under
// the plugin's folder, built with the NDK's clang into lib<Name>.so for
// each ABI. The plugin's Kotlin loads it with System.loadLibrary.
type nativeLib struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
	LDFlags []string `json:"ldflags"` // such as -lvulkan
}

// pluginNativeLibs builds the enabled plugins' C libraries for one ABI into
// dir, and returns each one's path by file name.
func (e *Env) pluginNativeLibs(root string, abi AndroidABI, dir string) (map[string]string, error) {
	names, err := enabledPlugins(root)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	for _, n := range names {
		m, err := e.pluginManifest(n)
		if err != nil {
			return nil, err
		}
		for _, l := range m.Android.Native {
			file := "lib" + l.Name + ".so"
			lib := filepath.Join(dir, file)
			e.step("cc → %s (android/%s)", file, abi.Name)
			if err := e.run(nil, e.ndkClang(abi), nativeArgs(filepath.Join(e.Repo, "plugins", n), l, lib)...); err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
			out[file] = lib
		}
	}
	return out, nil
}

func (e *Env) ndkClang(abi AndroidABI) string {
	return filepath.Join(e.NDK, "toolchains/llvm/prebuilt", ndkHost(), "bin", fmt.Sprintf("%s%d-clang", abi.Clang, MinAndroidAPI))
}

// nativeArgs are clang's arguments: a shared library, optimized, with its
// pages 16 KB-aligned as Google Play requires.
func nativeArgs(dir string, l nativeLib, out string) []string {
	args := []string{"-shared", "-fPIC", "-O2", "-Wall", "-I" + dir, "-o", out}
	for _, s := range l.Sources {
		args = append(args, filepath.Join(dir, filepath.FromSlash(s)))
	}
	args = append(args, l.LDFlags...)
	return append(args, "-Wl,-z,max-page-size=16384")
}
