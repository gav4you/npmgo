// Package gracefulfs is a Go translation of the npm package graceful-fs@4.2.11
// (https://github.com/isaacs/node-graceful-fs), a drop-in replacement for
// Node's fs module that trades EMFILE errors for slower file system access.
// The original copyright notice and ISC license are preserved in LICENSE.
//
// Like the original, it wraps an fs module and changes a few functions:
//
//   - open, readFile, writeFile, appendFile, copyFile and readdir put a call
//     that fails with EMFILE or ENFILE (too many open files) on a queue
//     shared by every loaded copy, and retry it with a growing delay (1.2
//     times the time spent retrying, at most 100 ms) until it succeeds or
//     has failed for 60 seconds; a successful close resets the delays.
//   - readdir sorts the names it returns (JavaScript's default sort).
//   - chown, fchown, lchown, chmod, fchmod and lchmod (and their Sync forms)
//     ignore ENOSYS, and EINVAL and EPERM when not running as root.
//   - stat, fstat and lstat (and their Sync forms) correct a negative uid or
//     gid by adding 2^32.
//   - read and readSync retry up to 10 times on EAGAIN.
//   - on win32, rename retries for up to 60 seconds while it fails with
//     EACCES, EPERM or EBUSY and the destination does not exist.
//   - lchmod, lchown and lutimes become no-ops where the module lacks them,
//     and lutimes is emulated with O_SYMLINK where the platform has it.
//   - the open of a ReadStream or WriteStream goes through the retrying open.
//
// The package has two layers.
//
// The translation itself is Runtime.Load: require('graceful-fs') over an fs
// module modelled as an FS value (a struct of the functions graceful-fs reads,
// nil where the module has none), on a Host that supplies Date.now, timers and
// process.nextTick. It keeps JavaScript's shapes: values graceful-fs passes on
// without looking at them are `any` (JavaScript's null is nil, undefined is
// jsvalue.Undefined{}), callbacks receive what the module delivered, and a
// callback given in the place of an optional argument is moved to the callback
// position as in JavaScript (readFile(path, cb) is ReadFile(path, cb, nil)).
// It runs single-threaded, like JavaScript: a Runtime and the FS values loaded
// on it must only be used from the goroutine that runs the Host's callbacks.
// The parity tests drive it over a scripted fs module on a virtual clock and
// compare every call it makes with the original's.
//
// The blocking functions (Open, Close, ReadFile, WriteFile, AppendFile,
// ReadDir, ReadDirEntries, Stat, Lstat, Fstat, Chmod, Fchmod, Lchmod, Chown,
// Fchown, Lchown, Rename, Read, CreateReadStream, CreateWriteStream) run that
// translation over the real file system on one internal event loop and wait
// for the result, so they may be called from any goroutine.
package gracefulfs

import (
	iofs "io/fs"
	"math"
)

// FS is an fs module: the functions graceful-fs reads from require('fs') and
// those it adds, with the arguments graceful-fs passes. A nil field is a
// function the module does not have; calling it panics, as calling undefined
// throws a TypeError in JavaScript.
//
// Asynchronous functions report through their callback, which a backend
// should call later, never before returning, as Node does. Synchronous
// functions return the error JavaScript would throw.
//
// An error's code (err.code: "EMFILE", "ENOENT", …) is read from a Code()
// string method, or translated from the syscall.Errno it wraps as libuv does.
type FS struct {
	// open(path, flags, mode, cb). A function given as mode is the callback
	// (mode is then null).
	Open func(path, flags, mode any, cb func(err error, fd any))
	// openSync(path, flags), used by the O_SYMLINK lutimesSync.
	OpenSync func(path, flags any) (fd any, err error)
	// close(fd, cb) and closeSync(fd).
	Close     func(fd any, cb func(err error))
	CloseSync func(fd any) error
	// readFile(path, options, cb), writeFile(path, data, options, cb),
	// appendFile(path, data, options, cb). A function given as options is
	// the callback (options is then null).
	ReadFile   func(path, options any, cb func(err error, data any))
	WriteFile  func(path, data, options any, cb func(err error))
	AppendFile func(path, data, options any, cb func(err error))
	// copyFile(src, dest, flags, cb). A function given as flags is the
	// callback (flags is then 0).
	CopyFile func(src, dest, flags any, cb func(err error))
	// readdir(path, options, cb). A function given as options is the callback
	// (options is then null). The names may be strings, []byte (Node's
	// Buffers, encoding 'buffer') or directory entries (withFileTypes).
	Readdir func(path, options any, cb func(err error, files []any))
	// read(fd, buffer, offset, length, position, cb) and readSync.
	Read     func(fd, buffer, offset, length, position any, cb func(err error, bytesRead, buffer any))
	ReadSync func(fd, buffer, offset, length, position any) (bytesRead any, err error)
	// rename(oldPath, newPath, cb).
	Rename func(oldPath, newPath any, cb func(err error))
	// chown(path, uid, gid, cb), fchown(fd, uid, gid, cb), lchown(path, …).
	Chown, Fchown, Lchown             func(target, uid, gid any, cb func(err error))
	ChownSync, FchownSync, LchownSync func(target, uid, gid any) error
	// chmod(path, mode, cb), fchmod(fd, mode, cb), lchmod(path, mode, cb).
	Chmod, Fchmod, Lchmod             func(target, mode any, cb func(err error))
	ChmodSync, FchmodSync, LchmodSync func(target, mode any) error
	// stat(path, options, cb), fstat(fd, …), lstat(path, …). graceful-fs
	// passes options only when it is truthy: nil options means the call had
	// none. A function given as options is the callback.
	Stat, Fstat, Lstat             func(target, options any, cb func(err error, stats *Stats))
	StatSync, FstatSync, LstatSync func(target, options any) (*Stats, error)
	// futimes(fd, atime, mtime, cb) and lutimes(path, atime, mtime, cb).
	Futimes     func(fd, atime, mtime any, cb func(err error))
	FutimesSync func(fd, atime, mtime any) error
	Lutimes     func(path, atime, mtime any, cb func(err error))
	LutimesSync func(path, atime, mtime any) error
	// ReadStream and WriteStream are the fs.ReadStream and fs.WriteStream
	// constructors. open is the stream's open method (nil: the class's own);
	// the constructor calls it to open the stream when no fd was given, as
	// Node does for a subclass that overrides open.
	ReadStream, WriteStream func(path, options any, open func(s Stream)) Stream
	// createReadStream(path, options) and createWriteStream(path, options).
	CreateReadStream, CreateWriteStream func(path, options any) Stream

	// gracefulQueue is fs[Symbol.for('graceful-fs.queue')]: set when the
	// first graceful-fs loaded over this module patched its close.
	gracefulQueue *queue
}

// Stats is a stat result as graceful-fs sees it: the uid and gid it corrects,
// and the file's information on a real file system (nil otherwise).
type Stats struct {
	Uid, Gid int64
	FileInfo iofs.FileInfo
}

// Stream is the part of a Node fs.ReadStream or fs.WriteStream that
// graceful-fs's open method uses.
type Stream interface {
	// OpenArgs returns stream.path, stream.flags and stream.mode.
	OpenArgs() (path, flags, mode any)
	// AutoClose returns stream.autoClose.
	AutoClose() bool
	// SetFD sets stream.fd.
	SetFD(fd any)
	// Destroy is stream.destroy().
	Destroy()
	// Emit is stream.emit(event, arg).
	Emit(event string, arg any)
	// Read is stream.read().
	Read()
}

// Host is the JavaScript host graceful-fs runs on: Date.now(), setTimeout,
// clearTimeout and process.nextTick. internal/jsloop's Virtual is a
// deterministic Host on a virtual clock.
type Host interface {
	// Now returns Date.now(): the time in whole milliseconds.
	Now() int64
	// SetTimeout calls fn ms milliseconds later (Node runs a delay below 1
	// ms after 1 ms) and returns a timer for ClearTimeout.
	SetTimeout(fn func(), ms int64) any
	// ClearTimeout cancels a timer; nil (undefined) is ignored.
	ClearTimeout(timer any)
	// NextTick calls fn after the current callback, before any timer.
	NextTick(fn func())
}

// Runtime is the process graceful-fs is loaded into.
type Runtime struct {
	// Host supplies the clock, the timers and process.nextTick.
	Host Host
	// Platform is graceful-fs's platform: the GRACEFUL_FS_PLATFORM
	// environment variable if set, otherwise process.platform ("win32",
	// "linux", "darwin", …), as polyfills.js reads it when it loads.
	Platform string
	// Getuid is process.getuid, read when a chown or chmod fails; nil where
	// Node has none (Windows), which counts as not root.
	Getuid func() int
	// OSymlink is constants.O_SYMLINK, 0 where the platform has none (all
	// but macOS).
	OSymlink int

	// global is global[Symbol.for('graceful-fs.queue')].
	global *queue
}

// queue is the shared retry queue.
type queue struct{ items []*entry }

// entry is one queued call: graceful-fs's [fn, args, err, startTime,
// lastTime]. retry is fn.apply(null, args.concat([startTime])); timeout is
// what the 60-second cutoff does with args' last element, the callback.
type entry struct {
	retry     func(startTime int64)
	timeout   func(err error)
	err       error
	startTime int64
	lastTime  int64
}

// module is one loaded copy of graceful-fs.js and its polyfills.js: its own
// retry timer, working on the queue of the fs module it was loaded over.
type module struct {
	rt         *Runtime
	q          *queue
	retryTimer any
	platform   string
	oSymlink   int
}

// Load is require('graceful-fs') in rt where require('fs') is base, each call
// a freshly loaded copy: it returns a patched copy of base and leaves base's
// other functions alone.
//
// The first copy loaded over a base also replaces base.Close and
// base.CloseSync, so that a successful close made anywhere through the module
// resets the queue, as graceful-fs patches the real fs module. Every copy
// works on that base's queue (which is rt's, unless the base was first loaded
// in another Runtime), and has its own retry timer.
func (rt *Runtime) Load(base *FS) *FS {
	m := &module{rt: rt, platform: rt.Platform, oSymlink: rt.OSymlink}
	// Once time initialization
	if base.gracefulQueue == nil {
		// This queue can be shared by multiple loaded instances
		q := rt.global
		if q == nil {
			q = &queue{}
		}
		base.gracefulQueue = q
		m.patchClose(base)
	}
	if rt.global == nil {
		rt.global = base.gracefulQueue
	}
	m.q = base.gracefulQueue
	clone := *base
	clone.gracefulQueue = nil // clone() copies string-keyed properties only
	return m.patch(&clone)
}

// platformOf is polyfills.js's platform: process.env.GRACEFUL_FS_PLATFORM ||
// process.platform.
func platformOf(env, processPlatform string) string {
	if env != "" {
		return env
	}
	return processPlatform
}

// patchClose replaces fs.close and fs.closeSync by versions that reset the
// shared queue: "we need to retry() whenever a close happens *anywhere* in
// the program".
func (m *module) patchClose(fs *FS) {
	fsClose := fs.Close
	fs.Close = func(fd any, cb func(err error)) {
		fsClose(fd, func(err error) {
			// This function uses the graceful-fs shared queue
			if err == nil {
				m.resetQueue()
			}
			if cb != nil {
				cb(err)
			}
		})
	}

	fsCloseSync := fs.CloseSync
	fs.CloseSync = func(fd any) error {
		// This function uses the graceful-fs shared queue
		if err := fsCloseSync(fd); err != nil {
			return err
		}
		m.resetQueue()
		return nil
	}
}

// emfile reports err.code === 'EMFILE' || err.code === 'ENFILE'.
func emfile(err error) bool {
	c := errCode(err)
	return c == "EMFILE" || c == "ENFILE"
}

// enqueueRetry queues [go$fn, args, err, startTime || Date.now(), Date.now()]:
// a startTime of 0 is unset, as JavaScript's || treats it.
func (m *module) enqueueRetry(err error, startTime int64, retry func(startTime int64), timeout func(err error)) {
	if startTime == 0 {
		startTime = m.rt.Host.Now()
	}
	m.enqueue(&entry{retry: retry, timeout: timeout, err: err, startTime: startTime, lastTime: m.rt.Host.Now()})
}

func (m *module) enqueue(elem *entry) {
	m.q.items = append(m.q.items, elem)
	m.retry()
}

// resetQueue resets the startTime and lastTime of every queued call to now:
// this restarts the 60-second cutoff and the delay between attempts, so the
// calls are retried sooner.
func (m *module) resetQueue() {
	now := m.rt.Host.Now()
	for _, elem := range m.q.items {
		elem.startTime = now
		elem.lastTime = now
	}
	// call retry to make sure we're actively processing the queue
	m.retry()
}

// retry takes the call at the head of the queue and retries it, fails it
// with its last error after 60 seconds, or puts it back at the end until its
// delay has passed; then it runs again on the next timer tick while the queue
// is not empty.
func (m *module) retry() {
	// clear the timer and remove it to help prevent unintended concurrency
	m.rt.Host.ClearTimeout(m.retryTimer)
	m.retryTimer = nil

	if len(m.q.items) == 0 {
		return
	}

	elem := m.q.items[0]
	m.q.items[0] = nil
	m.q.items = m.q.items[1:]

	if m.rt.Host.Now()-elem.startTime >= 60000 {
		// it's been more than 60 seconds total, bail now
		elem.timeout(elem.err)
	} else {
		// the amount of time between the last attempt and right now
		sinceAttempt := m.rt.Host.Now() - elem.lastTime
		// the amount of time between when we first tried, and when we last
		// tried, rounded up to at least 1
		sinceStart := max(elem.lastTime-elem.startTime, 1)
		// backoff. wait longer than the total time we've been retrying, but
		// only up to a maximum of 100ms
		desiredDelay := math.Min(float64(sinceStart)*1.2, 100)
		// it's been long enough since the last retry, do it again
		if float64(sinceAttempt) >= desiredDelay {
			elem.retry(elem.startTime)
		} else {
			// if we can't do this job yet, push it to the end of the queue
			// and let the next iteration check again
			m.q.items = append(m.q.items, elem)
		}
	}

	// schedule our next run if one isn't already scheduled
	if m.retryTimer == nil {
		m.retryTimer = m.rt.Host.SetTimeout(m.retry, 0)
	}
}

// patch is graceful-fs.js's patch(fs): the polyfills, then the queued
// functions and the streams. Everything that references the open() function
// needs to be in here.
func (m *module) patch(fs *FS) *FS {
	m.polyfills(fs)

	fs.CreateReadStream = func(path, options any) Stream { return fs.ReadStream(path, options, nil) }
	fs.CreateWriteStream = func(path, options any) Stream { return fs.WriteStream(path, options, nil) }

	fsReadFile := fs.ReadFile
	fs.ReadFile = func(path, options any, cb func(err error, data any)) {
		if c, ok := options.(func(error, any)); ok {
			cb, options = c, nil
		}
		var goReadFile func(path, options any, cb func(err error, data any), startTime int64)
		goReadFile = func(path, options any, cb func(err error, data any), startTime int64) {
			fsReadFile(path, options, func(err error, data any) {
				if err != nil && emfile(err) {
					m.enqueueRetry(err, startTime,
						func(s int64) { goReadFile(path, options, cb, s) },
						func(e error) {
							if cb != nil {
								cb(e, nil)
							}
						})
				} else if cb != nil {
					cb(err, data)
				}
			})
		}
		goReadFile(path, options, cb, 0)
	}

	fsWriteFile := fs.WriteFile
	fs.WriteFile = func(path, data, options any, cb func(err error)) {
		if c, ok := options.(func(error)); ok {
			cb, options = c, nil
		}
		var goWriteFile func(path, data, options any, cb func(err error), startTime int64)
		goWriteFile = func(path, data, options any, cb func(err error), startTime int64) {
			fsWriteFile(path, data, options, func(err error) {
				if err != nil && emfile(err) {
					m.enqueueRetry(err, startTime,
						func(s int64) { goWriteFile(path, data, options, cb, s) },
						func(e error) {
							if cb != nil {
								cb(e)
							}
						})
				} else if cb != nil {
					cb(err)
				}
			})
		}
		goWriteFile(path, data, options, cb, 0)
	}

	if fsAppendFile := fs.AppendFile; fsAppendFile != nil {
		fs.AppendFile = func(path, data, options any, cb func(err error)) {
			if c, ok := options.(func(error)); ok {
				cb, options = c, nil
			}
			var goAppendFile func(path, data, options any, cb func(err error), startTime int64)
			goAppendFile = func(path, data, options any, cb func(err error), startTime int64) {
				fsAppendFile(path, data, options, func(err error) {
					if err != nil && emfile(err) {
						m.enqueueRetry(err, startTime,
							func(s int64) { goAppendFile(path, data, options, cb, s) },
							func(e error) {
								if cb != nil {
									cb(e)
								}
							})
					} else if cb != nil {
						cb(err)
					}
				})
			}
			goAppendFile(path, data, options, cb, 0)
		}
	}

	if fsCopyFile := fs.CopyFile; fsCopyFile != nil {
		fs.CopyFile = func(src, dest, flags any, cb func(err error)) {
			if c, ok := flags.(func(error)); ok {
				cb, flags = c, 0
			}
			var goCopyFile func(src, dest, flags any, cb func(err error), startTime int64)
			goCopyFile = func(src, dest, flags any, cb func(err error), startTime int64) {
				fsCopyFile(src, dest, flags, func(err error) {
					if err != nil && emfile(err) {
						m.enqueueRetry(err, startTime,
							func(s int64) { goCopyFile(src, dest, flags, cb, s) },
							func(e error) {
								if cb != nil {
									cb(e)
								}
							})
					} else if cb != nil {
						cb(err)
					}
				})
			}
			goCopyFile(src, dest, flags, cb, 0)
		}
	}

	// readdir passes options on Node 6 and later (graceful-fs leaves them
	// out on Node 0 to 5).
	fsReaddir := fs.Readdir
	fs.Readdir = func(path, options any, cb func(err error, files []any)) {
		if c, ok := options.(func(error, []any)); ok {
			cb, options = c, nil
		}
		var goReaddir func(path, options any, cb func(err error, files []any), startTime int64)
		goReaddir = func(path, options any, cb func(err error, files []any), startTime int64) {
			fsReaddir(path, options, func(err error, files []any) {
				if err != nil && emfile(err) {
					m.enqueueRetry(err, startTime,
						func(s int64) { goReaddir(path, options, cb, s) },
						func(e error) {
							if cb != nil {
								cb(e, nil)
							}
						})
				} else {
					if files != nil {
						sortFiles(files)
					}
					if cb != nil {
						cb(err, files)
					}
				}
			})
		}
		goReaddir(path, options, cb, 0)
	}

	// open is declared here because the streams' open methods call it (the
	// function, not fs.open).
	var open func(path, flags, mode any, cb func(err error, fd any))

	readStreamOpen := func(that Stream) {
		path, flags, mode := that.OpenArgs()
		open(path, flags, mode, func(err error, fd any) {
			if err != nil {
				if that.AutoClose() {
					that.Destroy()
				}
				that.Emit("error", err)
			} else {
				that.SetFD(fd)
				that.Emit("open", fd)
				that.Read()
			}
		})
	}
	writeStreamOpen := func(that Stream) {
		path, flags, mode := that.OpenArgs()
		open(path, flags, mode, func(err error, fd any) {
			if err != nil {
				that.Destroy()
				that.Emit("error", err)
			} else {
				that.SetFD(fd)
				that.Emit("open", fd)
			}
		})
	}
	fsReadStream := fs.ReadStream
	fs.ReadStream = func(path, options any, openMethod func(s Stream)) Stream {
		if openMethod == nil {
			openMethod = readStreamOpen
		}
		return fsReadStream(path, options, openMethod)
	}
	fsWriteStream := fs.WriteStream
	fs.WriteStream = func(path, options any, openMethod func(s Stream)) Stream {
		if openMethod == nil {
			openMethod = writeStreamOpen
		}
		return fsWriteStream(path, options, openMethod)
	}

	fsOpen := fs.Open
	open = func(path, flags, mode any, cb func(err error, fd any)) {
		if c, ok := mode.(func(error, any)); ok {
			cb, mode = c, nil
		}
		var goOpen func(path, flags, mode any, cb func(err error, fd any), startTime int64)
		goOpen = func(path, flags, mode any, cb func(err error, fd any), startTime int64) {
			fsOpen(path, flags, mode, func(err error, fd any) {
				if err != nil && emfile(err) {
					m.enqueueRetry(err, startTime,
						func(s int64) { goOpen(path, flags, mode, cb, s) },
						func(e error) {
							if cb != nil {
								cb(e, nil)
							}
						})
				} else if cb != nil {
					cb(err, fd)
				}
			})
		}
		goOpen(path, flags, mode, cb, 0)
	}
	fs.Open = open

	return fs
}
