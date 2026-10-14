package gracefulfs

import "github.com/gav4you/npmgo/internal/jsvalue"

// polyfills is polyfills.js's patch(fs): (re-)implement some things that are
// known busted or missing.
//
// Not translated: the lchmod back-port for Node 0.5 and 0.6.0 to 0.6.2, and
// the replacement of process.cwd and process.chdir that polyfills.js makes
// when it loads (see PROGRESS.md).
func (m *module) polyfills(fs *FS) {
	// lutimes implementation, or no-op
	if fs.Lutimes == nil {
		m.patchLutimes(fs)
	}

	// https://github.com/isaacs/node-graceful-fs/issues/4
	// Chown should not fail on einval or eperm if non-root.
	// It should not fail on enosys ever, as this just indicates
	// that a fs doesn't support the intended operation.

	fs.Chown = m.chownFix(fs.Chown)
	fs.Fchown = m.chownFix(fs.Fchown)
	fs.Lchown = m.chownFix(fs.Lchown)

	fs.Chmod = m.chmodFix(fs.Chmod)
	fs.Fchmod = m.chmodFix(fs.Fchmod)
	fs.Lchmod = m.chmodFix(fs.Lchmod)

	fs.ChownSync = m.chownFixSync(fs.ChownSync)
	fs.FchownSync = m.chownFixSync(fs.FchownSync)
	fs.LchownSync = m.chownFixSync(fs.LchownSync)

	fs.ChmodSync = m.chmodFixSync(fs.ChmodSync)
	fs.FchmodSync = m.chmodFixSync(fs.FchmodSync)
	fs.LchmodSync = m.chmodFixSync(fs.LchmodSync)

	fs.Stat = statFix(fs.Stat)
	fs.Fstat = statFix(fs.Fstat)
	fs.Lstat = statFix(fs.Lstat)

	fs.StatSync = statFixSync(fs.StatSync)
	fs.FstatSync = statFixSync(fs.FstatSync)
	fs.LstatSync = statFixSync(fs.LstatSync)

	// if lchmod/lchown do not exist, then make them no-ops
	if fs.Chmod != nil && fs.Lchmod == nil {
		fs.Lchmod = func(path, mode any, cb func(err error)) {
			if cb != nil {
				m.rt.Host.NextTick(func() { cb(nil) })
			}
		}
		fs.LchmodSync = func(path, mode any) error { return nil }
	}
	if fs.Chown != nil && fs.Lchown == nil {
		fs.Lchown = func(path, uid, gid any, cb func(err error)) {
			if cb != nil {
				m.rt.Host.NextTick(func() { cb(nil) })
			}
		}
		fs.LchownSync = func(path, uid, gid any) error { return nil }
	}

	// on Windows, A/V software can lock the directory, causing this
	// to fail with an EACCES or EPERM if the directory contains newly
	// created files.  Try again on failure, for up to 60 seconds.

	// Set the timeout this long because some Windows Anti-Virus, such as Parity
	// bit9, may lock files for up to a minute, causing npm package install
	// failures. Also, take care to yield the scheduler. Windows scheduling gives
	// CPU to a busy looping process, which can cause the program causing the lock
	// contention to be starved of CPU by node, so the contention doesn't resolve.
	if m.platform == "win32" && fs.Rename != nil {
		fsRename := fs.Rename
		fs.Rename = func(from, to any, cb func(err error)) {
			start := m.rt.Host.Now()
			backoff := int64(0)
			var CB func(er error)
			CB = func(er error) {
				if er != nil {
					if c := errCode(er); (c == "EACCES" || c == "EPERM" || c == "EBUSY") && m.rt.Host.Now()-start < 60000 {
						m.rt.Host.SetTimeout(func() {
							// fs.stat(to, cb): the patched stat, looked up now; the
							// callback is passed in the options position, as in
							// JavaScript's two-argument call.
							fs.Stat(to, func(stater error, st *Stats) {
								if stater != nil && errCode(stater) == "ENOENT" {
									fsRename(from, to, CB)
								} else {
									cb(er) // not checked: a missing cb panics, as JavaScript throws
								}
							}, nil)
						}, backoff)
						if backoff < 100 {
							backoff += 10
						}
						return
					}
				}
				if cb != nil {
					cb(er)
				}
			}
			fsRename(from, to, CB)
		}
	}

	// if read() returns EAGAIN, then just try it again.
	if fsRead := fs.Read; fsRead != nil {
		fs.Read = func(fd, buffer, offset, length, position any, callback_ func(err error, bytesRead, buffer any)) {
			var callback func(err error, bytesRead, buffer any)
			if callback_ != nil {
				eagCounter := 0
				callback = func(er error, bytesRead, buf any) {
					if er != nil && errCode(er) == "EAGAIN" && eagCounter < 10 {
						eagCounter++
						fsRead(fd, buffer, offset, length, position, callback)
						return
					}
					callback_(er, bytesRead, buf)
				}
			}
			fsRead(fd, buffer, offset, length, position, callback)
		}
	}

	if fsReadSync := fs.ReadSync; fsReadSync != nil {
		fs.ReadSync = func(fd, buffer, offset, length, position any) (any, error) {
			eagCounter := 0
			for {
				n, er := fsReadSync(fd, buffer, offset, length, position)
				if er == nil {
					return n, nil
				}
				if errCode(er) == "EAGAIN" && eagCounter < 10 {
					eagCounter++
					continue
				}
				return nil, er
			}
		}
	}
}

// patchLutimes gives fs a lutimes: through open(O_SYMLINK), futimes and close
// where the platform has O_SYMLINK (macOS), otherwise a no-op, if fs has
// futimes at all. It looks up fs.open, fs.futimes and fs.close when called,
// so they are graceful-fs's own.
func (m *module) patchLutimes(fs *FS) {
	if m.oSymlink != 0 && fs.Futimes != nil {
		oSymlink := m.oSymlink
		fs.Lutimes = func(path, at, mt any, cb func(err error)) {
			fs.Open(path, oSymlink, func(er error, fd any) {
				if er != nil {
					if cb != nil {
						cb(er)
					}
					return
				}
				fs.Futimes(fd, at, mt, func(er error) {
					fs.Close(fd, func(er2 error) {
						if cb != nil {
							if er == nil {
								er = er2
							}
							cb(er)
						}
					})
				})
			}, nil)
		}

		fs.LutimesSync = func(path, at, mt any) error {
			fd, err := fs.OpenSync(path, oSymlink)
			if err != nil {
				return err
			}
			if err := fs.FutimesSync(fd, at, mt); err != nil {
				_ = fs.CloseSync(fd) // try { fs.closeSync(fd) } catch (er) {}
				return err
			}
			return fs.CloseSync(fd)
		}
	} else if fs.Futimes != nil {
		fs.Lutimes = func(_, _, _ any, cb func(err error)) {
			if cb != nil {
				m.rt.Host.NextTick(func() { cb(nil) })
			}
		}
		fs.LutimesSync = func(_, _, _ any) error { return nil }
	}
}

func (m *module) chmodFix(orig func(target, mode any, cb func(err error))) func(target, mode any, cb func(err error)) {
	if orig == nil {
		return nil
	}
	return func(target, mode any, cb func(err error)) {
		orig(target, mode, func(er error) {
			if m.chownErOk(er) {
				er = nil
			}
			if cb != nil {
				cb(er)
			}
		})
	}
}

func (m *module) chmodFixSync(orig func(target, mode any) error) func(target, mode any) error {
	if orig == nil {
		return nil
	}
	return func(target, mode any) error {
		if er := orig(target, mode); er != nil && !m.chownErOk(er) {
			return er
		}
		return nil
	}
}

func (m *module) chownFix(orig func(target, uid, gid any, cb func(err error))) func(target, uid, gid any, cb func(err error)) {
	if orig == nil {
		return nil
	}
	return func(target, uid, gid any, cb func(err error)) {
		orig(target, uid, gid, func(er error) {
			if m.chownErOk(er) {
				er = nil
			}
			if cb != nil {
				cb(er)
			}
		})
	}
}

func (m *module) chownFixSync(orig func(target, uid, gid any) error) func(target, uid, gid any) error {
	if orig == nil {
		return nil
	}
	return func(target, uid, gid any) error {
		if er := orig(target, uid, gid); er != nil && !m.chownErOk(er) {
			return er
		}
		return nil
	}
}

// fixStats corrects the signed uid and gid that older versions of Node
// erroneously returned.
func fixStats(stats *Stats) {
	if stats != nil {
		if stats.Uid < 0 {
			stats.Uid += 0x100000000
		}
		if stats.Gid < 0 {
			stats.Gid += 0x100000000
		}
	}
}

func statFix(orig func(target, options any, cb func(err error, stats *Stats))) func(target, options any, cb func(err error, stats *Stats)) {
	if orig == nil {
		return nil
	}
	return func(target, options any, cb func(err error, stats *Stats)) {
		if c, ok := options.(func(error, *Stats)); ok {
			cb, options = c, nil
		}
		callback := func(er error, stats *Stats) {
			fixStats(stats)
			if cb != nil {
				cb(er, stats)
			}
		}
		if jsvalue.Truthy(options) {
			orig(target, options, callback)
		} else {
			orig(target, nil, callback)
		}
	}
}

func statFixSync(orig func(target, options any) (*Stats, error)) func(target, options any) (*Stats, error) {
	if orig == nil {
		return nil
	}
	return func(target, options any) (*Stats, error) {
		if !jsvalue.Truthy(options) {
			options = nil
		}
		stats, err := orig(target, options)
		if err != nil {
			return nil, err
		}
		fixStats(stats)
		return stats, nil
	}
}

// chownErOk reports whether a chown or chmod error should be ignored.
//
// ENOSYS means that the fs doesn't support the op. Just ignore
// that, because it doesn't matter.
//
// if there's no getuid, or if getuid() is something other
// than 0, and the error is EINVAL or EPERM, then just ignore
// it.
//
// This specific case is a silent failure in cp, install, tar,
// and most other unix tools that manage permissions.
//
// When running as root, or if other types of errors are
// encountered, then it's strict.
func (m *module) chownErOk(er error) bool {
	if er == nil {
		return true
	}

	code := errCode(er)
	if code == "ENOSYS" {
		return true
	}

	nonroot := m.rt.Getuid == nil || m.rt.Getuid() != 0
	if nonroot {
		if code == "EINVAL" || code == "EPERM" {
			return true
		}
	}

	return false
}
