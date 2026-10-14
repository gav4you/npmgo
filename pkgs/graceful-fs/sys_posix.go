//go:build !windows && !plan9

package gracefulfs

import (
	"errors"
	"os"
	"syscall"
)

// sysErrCode returns the code Node gives the errno err wraps, for the codes
// graceful-fs looks at. On Unix libuv reports an errno under its own name
// (UV_EMFILE is -EMFILE), so this is a lookup of the errno's name; other
// errnos return "".
func sysErrCode(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return ""
	}
	switch errno {
	case syscall.EMFILE:
		return "EMFILE"
	case syscall.ENFILE:
		return "ENFILE"
	case syscall.EAGAIN:
		return "EAGAIN"
	case syscall.ENOSYS:
		return "ENOSYS"
	case syscall.EINVAL:
		return "EINVAL"
	case syscall.EPERM:
		return "EPERM"
	case syscall.EACCES:
		return "EACCES"
	case syscall.EBUSY:
		return "EBUSY"
	case syscall.ENOENT:
		return "ENOENT"
	}
	return ""
}

func chown(name string, uid, gid int) error     { return os.Chown(name, uid, gid) }
func fchown(f *os.File, uid, gid int) error     { return f.Chown(uid, gid) }
func lchown(name string, uid, gid int) error    { return os.Lchown(name, uid, gid) }
func processGetuid() func() int                 { return os.Getuid }
func statOwner(fi os.FileInfo) (uid, gid int64) { return sysOwner(fi.Sys()) }

// sysOwner reads the owner from a syscall.Stat_t.
func sysOwner(sys any) (uid, gid int64) {
	if st, ok := sys.(*syscall.Stat_t); ok {
		return int64(st.Uid), int64(st.Gid)
	}
	return 0, 0
}
