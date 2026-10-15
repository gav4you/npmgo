//go:build !windows && !plan9

package gracefulfs

import (
	"os"
	"syscall"
	"testing"
)

// On Unix sysErrCode names the errno, for the nine codes graceful-fs reads;
// any other errno has no code. (Plan 9 has no syscall.Errno, so this file
// stays out of its build.)
func TestErrCodeErrno(t *testing.T) {
	for e, want := range map[syscall.Errno]string{
		syscall.EMFILE: "EMFILE", syscall.ENFILE: "ENFILE", syscall.EAGAIN: "EAGAIN",
		syscall.ENOSYS: "ENOSYS", syscall.EINVAL: "EINVAL", syscall.EPERM: "EPERM",
		syscall.EACCES: "EACCES", syscall.EBUSY: "EBUSY", syscall.ENOENT: "ENOENT",
	} {
		if got := errCode(&os.PathError{Op: "x", Path: "y", Err: e}); got != want {
			t.Errorf("errCode(%v) = %q, want %q", e, got, want)
		}
	}
	if got := errCode(syscall.EEXIST); got != "" {
		t.Errorf("errCode(EEXIST) = %q, want no code", got)
	}
}
