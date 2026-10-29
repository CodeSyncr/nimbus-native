package toolchain

import "strings"

// goBuild compiles the Go app in dir as a C library (c-archive for iOS,
// c-shared for Android) using the given GOOS/GOARCH/CC environment.
func (e *Env) goBuild(dir string, env []string, mode, out string, ldflags ...string) error {
	args := []string{"build", "-buildmode=" + mode, "-trimpath", "-o", out}
	if len(ldflags) > 0 {
		args = append(args, "-ldflags="+strings.Join(ldflags, " "))
	}
	return e.runIn(dir, env, "go", append(args, ".")...)
}
