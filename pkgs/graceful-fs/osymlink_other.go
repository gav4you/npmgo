//go:build !darwin

package gracefulfs

// oSymlink is 0: only macOS has O_SYMLINK.
const oSymlink = 0
