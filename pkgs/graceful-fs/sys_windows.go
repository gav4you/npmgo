//go:build windows

package gracefulfs

import "os"

// chown, fchown and lchown succeed without doing anything, as libuv's
// fs__chown, fs__fchown and fs__lchown do on Windows (package os would
// report "not supported by windows").
func chown(name string, uid, gid int) error  { return nil }
func fchown(f *os.File, uid, gid int) error  { return nil }
func lchown(name string, uid, gid int) error { return nil }

// processGetuid is nil: Node has no process.getuid on Windows, which
// graceful-fs counts as not root.
func processGetuid() func() int { return nil }

// statOwner is 0, 0: Node reports uid and gid 0 on Windows.
func statOwner(fi os.FileInfo) (uid, gid int64) { return 0, 0 }
