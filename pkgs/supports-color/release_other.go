//go:build !windows

package supportscolor

// osRelease is left empty outside Windows: supports-color reads os.release()
// only when process.platform is win32.
func osRelease() string { return "" }
