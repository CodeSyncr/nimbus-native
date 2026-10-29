package toolchain

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The Kotlin compiler Android hosts are built with. It is downloaded once
// into the user cache and checked against this hash, so every machine
// builds with the same compiler.
const (
	kotlinVersion = "2.0.21"
	kotlinSHA256  = "0352c0a45bd22f80f6b26e485cd04da8047baa5de54865281fb9f89a4a7bcf2a"
	kotlinURL     = "https://github.com/JetBrains/kotlin/releases/download/v" + kotlinVersion + "/kotlin-compiler-" + kotlinVersion + ".zip"
)

// kotlinHome is where the pinned compiler lives once installed.
func kotlinHome() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "nimbus-native", "kotlin-"+kotlinVersion)
}

// Kotlinc returns the kotlinc to build with: NIMBUS_KOTLINC if set, else the
// pinned compiler, downloading and verifying it on first use.
func (e *Env) Kotlinc() (string, error) {
	if p := os.Getenv("NIMBUS_KOTLINC"); p != "" {
		return p, nil
	}
	bin := filepath.Join(kotlinHome(), "kotlinc", "bin", "kotlinc")
	if exists(bin) {
		return bin, nil
	}
	e.step("download Kotlin %s (once, ~80 MB)", kotlinVersion)
	if err := installKotlin(kotlinHome()); err != nil {
		return "", fmt.Errorf("installing Kotlin %s: %w", kotlinVersion, err)
	}
	return bin, nil
}

// InstalledKotlinc is the kotlinc that Kotlinc would use, without
// downloading: "" if it is not installed yet.
func InstalledKotlinc() string {
	if p := os.Getenv("NIMBUS_KOTLINC"); p != "" {
		return p
	}
	if bin := filepath.Join(kotlinHome(), "kotlinc", "bin", "kotlinc"); exists(bin) {
		return bin
	}
	return ""
}

// kotlinStdlib is the Kotlin standard library jar that goes into the APK.
func kotlinStdlib(kotlinc string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(kotlinc)), "lib", "kotlin-stdlib.jar")
}

func installKotlin(home string) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	zipPath := filepath.Join(home, "kotlin-compiler.zip")
	defer os.Remove(zipPath)

	resp, err := http.Get(kotlinURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", kotlinURL, resp.Status)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != kotlinSHA256 {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, kotlinSHA256)
	}

	// Unpack into a temporary directory, then move it into place, so an
	// interrupted install never looks complete.
	tmp, err := os.MkdirTemp(home, "unpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := unzip(zipPath, tmp); err != nil {
		return err
	}
	return os.Rename(filepath.Join(tmp, "kotlinc"), filepath.Join(home, "kotlinc"))
}

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	root := filepath.Clean(dst) + string(os.PathSeparator)
	for _, f := range r.File {
		p := filepath.Join(dst, f.Name)
		if !strings.HasPrefix(p, root) {
			return fmt.Errorf("zip entry %q escapes the destination", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.Contains(f.Name, "/bin/") {
			mode = 0o755
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
