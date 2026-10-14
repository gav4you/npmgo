package gracefulfs

import (
	"io"
	"os"
	"runtime"
)

// realFS is graceful-fs loaded over an fs module on a realHost.
type realFS struct {
	host *realHost
	rt   *Runtime
	fs   *FS
}

// std is graceful-fs loaded over the real file system, the module the
// blocking functions use. Like require('graceful-fs'), it reads
// GRACEFUL_FS_PLATFORM when the package is initialised.
var std = newRealFS(nil)

// newRealFS loads graceful-fs on a new realHost over base, or over the real
// file system if base is nil.
func newRealFS(base func(post func(func())) *FS) *realFS {
	h := newRealHost()
	if base == nil {
		base = osFS
	}
	rt := &Runtime{
		Host:     h,
		Platform: platformOf(os.Getenv("GRACEFUL_FS_PLATFORM"), nodePlatform(runtime.GOOS)),
		Getuid:   processGetuid(),
		OSymlink: oSymlink,
	}
	return &realFS{host: h, rt: rt, fs: rt.Load(base(h.post))}
}

// nodePlatform is process.platform for a GOOS.
func nodePlatform(goos string) string {
	switch goos {
	case "windows":
		return "win32"
	case "solaris", "illumos":
		return "sunos"
	}
	return goos
}

// call runs op on the event loop and waits until it calls done.
func (r *realFS) call(op func(done func())) {
	ch := make(chan struct{})
	r.host.post(func() { op(func() { close(ch) }) })
	<-ch
}

// errCall runs an operation that reports only an error.
func (r *realFS) errCall(op func(cb func(error))) error {
	var err error
	r.call(func(done func()) {
		op(func(e error) { err = e; done() })
	})
	return err
}

func (r *realFS) open(name string, flag int, perm os.FileMode) (*os.File, error) {
	var f *os.File
	var err error
	r.call(func(done func()) {
		r.fs.Open(name, flag, perm, func(e error, fd any) {
			err = e
			if e == nil {
				f = fd.(*os.File)
			}
			done()
		})
	})
	return f, err
}

func (r *realFS) close(f *os.File) error {
	return r.errCall(func(cb func(error)) { r.fs.Close(f, cb) })
}

func (r *realFS) readDir(name string, options any) ([]any, error) {
	var files []any
	var err error
	r.call(func(done func()) {
		r.fs.Readdir(name, options, func(e error, fl []any) { files, err = fl, e; done() })
	})
	return files, err
}

func (r *realFS) stat(st func(target, options any, cb func(error, *Stats)), target any) (os.FileInfo, error) {
	var fi os.FileInfo
	var err error
	r.call(func(done func()) {
		st(target, nil, func(e error, s *Stats) {
			err = e
			if e == nil {
				fi = s.FileInfo
			}
			done()
		})
	})
	return fi, err
}

func (r *realFS) createStream(create func(path, options any) Stream, name string) (*os.File, error) {
	var s *osStream
	r.call(func(done func()) { s = create(name, nil).(*osStream); done() })
	<-s.ready
	if s.err != nil {
		return nil, s.err
	}
	return s.f, nil
}

// Open opens a file like os.OpenFile, through graceful-fs's open: while the
// open fails with EMFILE or ENFILE it waits on the shared queue and retries,
// for up to 60 seconds after its first failure (a successful Close starts
// that period again). flag is os.O_* flags.
func Open(name string, flag int, perm os.FileMode) (*os.File, error) {
	return std.open(name, flag, perm)
}

// Close closes f through graceful-fs's close: when it succeeds, the calls
// waiting on the queue are retried sooner. Closing a file directly (f.Close)
// works but does not wake the queue.
func Close(f *os.File) error { return std.close(f) }

// ReadFile reads a file like os.ReadFile, retrying on EMFILE or ENFILE.
func ReadFile(name string) ([]byte, error) {
	var data []byte
	var err error
	std.call(func(done func()) {
		std.fs.ReadFile(name, nil, func(e error, d any) {
			err = e
			if e == nil {
				data = d.([]byte)
			}
			done()
		})
	})
	return data, err
}

// WriteFile writes a file like os.WriteFile, retrying on EMFILE or ENFILE.
func WriteFile(name string, data []byte, perm os.FileMode) error {
	return std.errCall(func(cb func(error)) { std.fs.WriteFile(name, data, perm, cb) })
}

// AppendFile appends data to a file, creating it with perm if needed, like
// Node's fs.appendFile, retrying on EMFILE or ENFILE.
func AppendFile(name string, data []byte, perm os.FileMode) error {
	return std.errCall(func(cb func(error)) { std.fs.AppendFile(name, data, perm, cb) })
}

// ReadDir returns the names in a directory (without "." and ".."), sorted as
// graceful-fs sorts them: by UTF-16 code units, as JavaScript sorts strings,
// which differs from Go's byte order for names with characters above U+FFFF.
// It retries on EMFILE or ENFILE.
func ReadDir(name string) ([]string, error) {
	files, err := std.readDir(name, nil)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.(string)
	}
	return names, nil
}

// ReadDirEntries returns the entries of a directory as readdir with
// withFileTypes does: graceful-fs leaves them in the order the file system
// gave them (strcmp order on Unix, as libuv sorts them; directory order on
// Windows), since the sort compares their string form "[object Object]". It
// retries on EMFILE or ENFILE.
func ReadDirEntries(name string) ([]os.DirEntry, error) {
	files, err := std.readDir(name, readdirWithFileTypes{})
	if err != nil {
		return nil, err
	}
	entries := make([]os.DirEntry, len(files))
	for i, f := range files {
		entries[i] = f.(os.DirEntry)
	}
	return entries, nil
}

// Stat is os.Stat through graceful-fs's stat.
func Stat(name string) (os.FileInfo, error) { return std.stat(std.fs.Stat, name) }

// Lstat is os.Lstat through graceful-fs's lstat.
func Lstat(name string) (os.FileInfo, error) { return std.stat(std.fs.Lstat, name) }

// Fstat is f.Stat through graceful-fs's fstat.
func Fstat(f *os.File) (os.FileInfo, error) { return std.stat(std.fs.Fstat, f) }

// Chmod is os.Chmod through graceful-fs's chmod, which ignores ENOSYS, and
// EINVAL and EPERM when the process is not root (or on Windows).
func Chmod(name string, mode os.FileMode) error {
	return std.errCall(func(cb func(error)) { std.fs.Chmod(name, mode, cb) })
}

// Fchmod is f.Chmod through graceful-fs's fchmod (see Chmod). On Windows
// package os changes the read-only attribute through f's own handle, which
// must be open for writing (Node reopens the handle for attribute access);
// on a file opened read-only the change fails with EPERM, which graceful-fs
// ignores because no Windows process counts as root.
func Fchmod(f *os.File, mode os.FileMode) error {
	return std.errCall(func(cb func(error)) { std.fs.Fchmod(f, mode, cb) })
}

// Lchmod is graceful-fs's lchmod: a no-op, as graceful-fs makes it where the
// fs module has no lchmod (Node has one on macOS only; this package has none).
func Lchmod(name string, mode os.FileMode) error {
	return std.errCall(func(cb func(error)) { std.fs.Lchmod(name, mode, cb) })
}

// Chown is os.Chown through graceful-fs's chown, which ignores ENOSYS, and
// EINVAL and EPERM when the process is not root. On Windows it does nothing,
// as Node's chown.
func Chown(name string, uid, gid int) error {
	return std.errCall(func(cb func(error)) { std.fs.Chown(name, uid, gid, cb) })
}

// Fchown is f.Chown through graceful-fs's fchown (see Chown).
func Fchown(f *os.File, uid, gid int) error {
	return std.errCall(func(cb func(error)) { std.fs.Fchown(f, uid, gid, cb) })
}

// Lchown is os.Lchown through graceful-fs's lchown (see Chown).
func Lchown(name string, uid, gid int) error {
	return std.errCall(func(cb func(error)) { std.fs.Lchown(name, uid, gid, cb) })
}

// Rename is os.Rename through graceful-fs's rename. On Windows (or with
// GRACEFUL_FS_PLATFORM=win32) a rename that fails with EACCES, EPERM or EBUSY,
// as when anti-virus software holds a file, is retried after 0, 10, 20 … 100
// ms while the destination does not exist, for up to 60 seconds.
func Rename(oldpath, newpath string) error {
	return std.errCall(func(cb func(error)) { std.fs.Rename(oldpath, newpath, cb) })
}

// Read reads from f at its current offset like f.Read, through graceful-fs's
// read, which retries up to 10 times on EAGAIN. At the end of the file it
// returns 0 and io.EOF.
func Read(f *os.File, b []byte) (int, error) {
	var n int
	var err error
	std.call(func(done func()) {
		std.fs.Read(f, b, 0, len(b), nil, func(e error, bytesRead, _ any) {
			err = e
			if e == nil {
				n = bytesRead.(int)
			}
			done()
		})
	})
	if err == nil && n == 0 && len(b) > 0 {
		err = io.EOF
	}
	return n, err
}

// CreateReadStream opens a file for reading as Node's fs.createReadStream
// does (flags "r", mode 0666), through graceful-fs's retrying open, and
// returns it.
func CreateReadStream(name string) (*os.File, error) {
	return std.createStream(std.fs.CreateReadStream, name)
}

// CreateWriteStream opens a file for writing as Node's fs.createWriteStream
// does (flags "w", mode 0666), through graceful-fs's retrying open, and
// returns it.
func CreateWriteStream(name string) (*os.File, error) {
	return std.createStream(std.fs.CreateWriteStream, name)
}
