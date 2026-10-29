package bridge

/*
#cgo LDFLAGS: -llog
#include <stdlib.h>
#include <android/log.h>
*/
import "C"

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	logTag = C.CString("NimbusNative")
	appTag = C.CString("NimbusNativeApp")
)

// Logf writes a line to logcat under the tag NimbusNative.
func Logf(format string, args ...any) {
	msg := C.CString(fmt.Sprintf(format, args...))
	defer C.free(unsafe.Pointer(msg))
	C.__android_log_write(C.ANDROID_LOG_INFO, logTag, msg)
}

// Android sends a process's stdout and stderr nowhere. Point both at pipes
// whose lines go to logcat (tag NimbusNativeApp), so fmt.Println and log
// output show up in `nimbus native run android` and Android Studio.
func init() {
	toLogcat(1, C.ANDROID_LOG_INFO)
	toLogcat(2, C.ANDROID_LOG_WARN)
}

func toLogcat(fd int, prio C.int) {
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	if err := syscall.Dup3(int(w.Fd()), fd, 0); err != nil {
		r.Close()
		w.Close()
		return
	}
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 4096), 64<<10)
		for sc.Scan() {
			msg := C.CString(sc.Text())
			C.__android_log_write(prio, appTag, msg)
			C.free(unsafe.Pointer(msg))
		}
	}()
}
