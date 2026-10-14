//go:build windows

package gracefulfs

import (
	"os"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// Spot checks of the libuv translation, above all the errors graceful-fs
// reacts to: an anti-virus lock is a sharing or lock violation (EBUSY) or
// access denied (EPERM), which win32's rename retries.
func TestUVTranslateSysError(t *testing.T) {
	for errno, want := range map[syscall.Errno]string{
		0:                                 "",
		windows.ERROR_TOO_MANY_OPEN_FILES: "EMFILE",
		windows.WSAEMFILE:                 "EMFILE",
		windows.ERROR_SHARING_VIOLATION:   "EBUSY",
		windows.ERROR_LOCK_VIOLATION:      "EBUSY",
		windows.ERROR_ACCESS_DENIED:       "EPERM",
		windows.ERROR_PRIVILEGE_NOT_HELD:  "EPERM",
		windows.ERROR_CANT_ACCESS_FILE:    "EACCES",
		windows.ERROR_FILE_NOT_FOUND:      "ENOENT",
		windows.ERROR_PATH_NOT_FOUND:      "ENOENT",
		windows.ERROR_INVALID_NAME:        "ENOENT",
		windows.ERROR_NO_DATA:             "EAGAIN",
		windows.WSAEWOULDBLOCK:            "EAGAIN",
		windows.ERROR_INVALID_PARAMETER:   "EINVAL",
		windows.ERROR_NOT_SUPPORTED:       "ENOTSUP",
		windows.ERROR_INVALID_FUNCTION:    "EISDIR",
		windows.ERROR_BROKEN_PIPE:         "EOF",
		windows.ERROR_BAD_EXE_FORMAT:      "EFTYPE",
		windows.ERROR_FILE_EXISTS:         "EEXIST",
		12345:                             "UNKNOWN",
		syscall.EINVAL:                    "UNKNOWN", // Go's invented errno, not a Windows error
		syscall.EWINDOWS:                  "UNKNOWN",
	} {
		if got := uvTranslateSysError(errno); got != want {
			t.Errorf("uvTranslateSysError(%d) = %q, want %q", errno, got, want)
		}
	}
	if got := errCode(&os.PathError{Op: "open", Path: "x", Err: windows.ERROR_SHARING_VIOLATION}); got != "EBUSY" {
		t.Errorf("errCode of a sharing violation = %q", got)
	}
}
