package gracefulfs

import (
	"errors"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
)

// osFS is the fs module over package os that the blocking functions use. Each
// asynchronous function does its work on a new goroutine (Node uses libuv's
// thread pool) and delivers its callback through post; a nil callback drops
// the result. It follows libuv where Node's results would differ from
// package os's: readdir lists names in strcmp order on Unix (libuv's scandir
// sorts them; on Windows they stay in directory order), chown, fchown and
// lchown succeed without doing anything on Windows, a read at the end of a
// file returns 0 bytes rather than io.EOF, and stat reports uid and gid 0
// on Windows.
//
// Values are Go's: paths are strings, file descriptors *os.File, flags
// os.O_* values, modes os.FileMode (nil for the default, 0666), data []byte.
// copyFile, lchmod, futimes, lutimes and their Sync forms are absent, so
// graceful-fs leaves copyFile and lutimes absent and makes lchmod a no-op.
func osFS(post func(func())) *FS {
	async := func(work func() func()) {
		go func() {
			deliver := work()
			post(deliver)
		}()
	}
	done := func(cb func(error), err error) func() {
		return func() {
			if cb != nil {
				cb(err)
			}
		}
	}
	return &FS{
		Open: func(path, flags, mode any, cb func(err error, fd any)) {
			name, flag, perm := path.(string), flags.(int), fileMode(mode)
			async(func() func() {
				f, err := os.OpenFile(name, flag, perm)
				return func() {
					if cb == nil {
						if f != nil {
							f.Close()
						}
						return
					}
					if err != nil {
						cb(err, nil)
						return
					}
					cb(nil, f)
				}
			})
		},
		OpenSync: func(path, flags any) (any, error) {
			f, err := os.OpenFile(path.(string), flags.(int), 0o666)
			if err != nil {
				return nil, err
			}
			return f, nil
		},
		Close: func(fd any, cb func(err error)) {
			f := fd.(*os.File)
			async(func() func() { return done(cb, f.Close()) })
		},
		CloseSync: func(fd any) error { return fd.(*os.File).Close() },
		ReadFile: func(path, options any, cb func(err error, data any)) {
			name := path.(string)
			async(func() func() {
				b, err := os.ReadFile(name)
				return func() {
					if cb == nil {
						return
					}
					if err != nil {
						cb(err, nil)
						return
					}
					cb(nil, b)
				}
			})
		},
		WriteFile: func(path, data, options any, cb func(err error)) {
			name, b, perm := path.(string), data.([]byte), fileMode(options)
			async(func() func() { return done(cb, os.WriteFile(name, b, perm)) })
		},
		AppendFile: func(path, data, options any, cb func(err error)) {
			name, b, perm := path.(string), data.([]byte), fileMode(options)
			async(func() func() {
				f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
				if err == nil {
					_, err = f.Write(b)
					if cerr := f.Close(); err == nil {
						err = cerr
					}
				}
				return done(cb, err)
			})
		},
		Readdir: func(path, options any, cb func(err error, files []any)) {
			name := path.(string)
			_, withFileTypes := options.(readdirWithFileTypes)
			async(func() func() {
				files, err := readDir(name, withFileTypes)
				return func() {
					if cb == nil {
						return
					}
					if err != nil {
						cb(err, nil)
						return
					}
					cb(nil, files)
				}
			})
		},
		Read: func(fd, buffer, offset, length, position any, cb func(err error, bytesRead, buffer any)) {
			f, b := fd.(*os.File), buffer.([]byte)
			off, n := offset.(int), length.(int)
			async(func() func() {
				read, err := readAt(f, b[off:off+n], position)
				return func() {
					if cb == nil {
						return
					}
					if err != nil {
						cb(err, nil, nil)
						return
					}
					cb(nil, read, b)
				}
			})
		},
		ReadSync: func(fd, buffer, offset, length, position any) (any, error) {
			f, b := fd.(*os.File), buffer.([]byte)
			off, n := offset.(int), length.(int)
			read, err := readAt(f, b[off:off+n], position)
			if err != nil {
				return nil, err
			}
			return read, nil
		},
		Rename: func(oldPath, newPath any, cb func(err error)) {
			from, to := oldPath.(string), newPath.(string)
			async(func() func() { return done(cb, os.Rename(from, to)) })
		},
		Chown: func(target, uid, gid any, cb func(err error)) {
			name, u, g := target.(string), uid.(int), gid.(int)
			async(func() func() { return done(cb, chown(name, u, g)) })
		},
		Fchown: func(target, uid, gid any, cb func(err error)) {
			f, u, g := target.(*os.File), uid.(int), gid.(int)
			async(func() func() { return done(cb, fchown(f, u, g)) })
		},
		Lchown: func(target, uid, gid any, cb func(err error)) {
			name, u, g := target.(string), uid.(int), gid.(int)
			async(func() func() { return done(cb, lchown(name, u, g)) })
		},
		ChownSync:  func(target, uid, gid any) error { return chown(target.(string), uid.(int), gid.(int)) },
		FchownSync: func(target, uid, gid any) error { return fchown(target.(*os.File), uid.(int), gid.(int)) },
		LchownSync: func(target, uid, gid any) error { return lchown(target.(string), uid.(int), gid.(int)) },
		Chmod: func(target, mode any, cb func(err error)) {
			name, perm := target.(string), fileMode(mode)
			async(func() func() { return done(cb, os.Chmod(name, perm)) })
		},
		Fchmod: func(target, mode any, cb func(err error)) {
			f, perm := target.(*os.File), fileMode(mode)
			async(func() func() { return done(cb, f.Chmod(perm)) })
		},
		ChmodSync:  func(target, mode any) error { return os.Chmod(target.(string), fileMode(mode)) },
		FchmodSync: func(target, mode any) error { return target.(*os.File).Chmod(fileMode(mode)) },
		Stat: func(target, options any, cb func(err error, stats *Stats)) {
			name := target.(string)
			async(func() func() { return deliverStats(cb)(newStats(os.Stat(name))) })
		},
		Lstat: func(target, options any, cb func(err error, stats *Stats)) {
			name := target.(string)
			async(func() func() { return deliverStats(cb)(newStats(os.Lstat(name))) })
		},
		Fstat: func(target, options any, cb func(err error, stats *Stats)) {
			f := target.(*os.File)
			async(func() func() { return deliverStats(cb)(newStats(f.Stat())) })
		},
		StatSync:  func(target, options any) (*Stats, error) { return newStats(os.Stat(target.(string))) },
		LstatSync: func(target, options any) (*Stats, error) { return newStats(os.Lstat(target.(string))) },
		FstatSync: func(target, options any) (*Stats, error) { return newStats(target.(*os.File).Stat()) },
		ReadStream: func(path, options any, open func(s Stream)) Stream {
			return newOSStream(path.(string), os.O_RDONLY, open)
		},
		WriteStream: func(path, options any, open func(s Stream)) Stream {
			return newOSStream(path.(string), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, open)
		},
	}
}

// readdirWithFileTypes is the readdir option {withFileTypes: true}.
type readdirWithFileTypes struct{}

// readDir lists a directory as Node's readdir does: names, or os.DirEntry
// values with withFileTypes, in strcmp order except on Windows.
func readDir(name string, withFileTypes bool) ([]any, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var files []any
	if withFileTypes {
		entries, err := f.ReadDir(-1)
		if err != nil {
			return nil, err
		}
		if runtime.GOOS != "windows" {
			slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
		}
		for _, e := range entries {
			files = append(files, e)
		}
	} else {
		names, err := f.Readdirnames(-1)
		if err != nil {
			return nil, err
		}
		if runtime.GOOS != "windows" {
			slices.Sort(names)
		}
		for _, n := range names {
			files = append(files, n)
		}
	}
	if files == nil {
		files = []any{}
	}
	return files, nil
}

// readAt reads at position, or at the file's offset when position is nil,
// undefined or negative (Node's null and -1). The end of the file is 0 bytes
// and no error, as in Node.
func readAt(f *os.File, b []byte, position any) (int, error) {
	var n int
	var err error
	if pos, ok := position.(int64); ok && pos >= 0 {
		n, err = f.ReadAt(b, pos)
	} else {
		n, err = f.Read(b)
	}
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return n, err
}

// fileMode is a mode argument: an os.FileMode, or nil or undefined for 0666.
func fileMode(mode any) os.FileMode {
	if m, ok := mode.(os.FileMode); ok {
		return m
	}
	return 0o666
}

func newStats(fi os.FileInfo, err error) (*Stats, error) {
	if err != nil {
		return nil, err
	}
	uid, gid := statOwner(fi)
	return &Stats{Uid: uid, Gid: gid, FileInfo: fi}, nil
}

// deliverStats returns a function that turns a stat result into the callback
// to run on the loop.
func deliverStats(cb func(error, *Stats)) func(*Stats, error) func() {
	return func(st *Stats, err error) func() {
		return func() {
			if cb != nil {
				cb(err, st)
			}
		}
	}
}

// osStream is a ReadStream or WriteStream over package os: graceful-fs's
// open method opens it, and it reports the outcome on ready.
type osStream struct {
	path   string
	flags  int
	f      *os.File
	err    error
	ready  chan struct{}
	opened bool
}

func newOSStream(path string, flags int, open func(s Stream)) *osStream {
	s := &osStream{path: path, flags: flags, ready: make(chan struct{})}
	if open == nil {
		open = func(s Stream) {
			st := s.(*osStream)
			f, err := os.OpenFile(st.path, st.flags, 0o666)
			if err != nil {
				st.Emit("error", err)
				return
			}
			st.SetFD(f)
			st.Emit("open", f)
		}
	}
	open(s)
	return s
}

func (s *osStream) OpenArgs() (path, flags, mode any) { return s.path, s.flags, os.FileMode(0o666) }
func (s *osStream) AutoClose() bool                   { return true }
func (s *osStream) SetFD(fd any)                      { s.f = fd.(*os.File) }
func (s *osStream) Read()                             {}

// Destroy closes the file if it was opened.
func (s *osStream) Destroy() {
	if s.f != nil {
		s.f.Close()
	}
}

// Emit records the outcome of the open: "open" or "error".
func (s *osStream) Emit(event string, arg any) {
	if s.opened {
		return
	}
	switch event {
	case "open":
		s.opened = true
		close(s.ready)
	case "error":
		s.opened = true
		s.err, _ = arg.(error)
		close(s.ready)
	}
}
