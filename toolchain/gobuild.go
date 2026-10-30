package toolchain

import (
	"os"
	"strings"
)

// goBuild compiles the Go app in dir as a C library (c-archive for iOS,
// c-shared for Android) using the given GOOS/GOARCH/CC environment.
func (e *Env) goBuild(dir string, env []string, mode, out string, ldflags ...string) error {
	args := []string{"build", "-buildmode=" + mode, "-trimpath", "-o", out}
	// Xcode and Gradle hand their environment down to `nimbus native lib`.
	if u := os.Getenv(serverEnv); u != "" {
		ldflags = append(ldflags, "-X", nativeModule+"/api.builtServerURL="+u)
	}
	if len(ldflags) > 0 {
		args = append(args, "-ldflags="+strings.Join(ldflags, " "))
	}
	return e.runIn(dir, env, "go", append(args, ".")...)
}
