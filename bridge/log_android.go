package bridge

/*
#cgo LDFLAGS: -llog
#include <stdlib.h>
#include <android/log.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

var logTag = C.CString("NimbusNative")

// Logf writes a line to logcat under the tag NimbusNative.
func Logf(format string, args ...any) {
	msg := C.CString(fmt.Sprintf(format, args...))
	defer C.free(unsafe.Pointer(msg))
	C.__android_log_write(C.ANDROID_LOG_INFO, logTag, msg)
}
