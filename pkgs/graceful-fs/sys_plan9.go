//go:build plan9

package gracefulfs

import "os"

// sysErrCode returns "": Plan 9 reports errors as strings, and Node does not
// run there.
func sysErrCode(err error) string { return "" }

func chown(name string, uid, gid int) error     { return os.Chown(name, uid, gid) }
func fchown(f *os.File, uid, gid int) error     { return f.Chown(uid, gid) }
func lchown(name string, uid, gid int) error    { return os.Lchown(name, uid, gid) }
func processGetuid() func() int                 { return os.Getuid }
func statOwner(fi os.FileInfo) (uid, gid int64) { return 0, 0 }
