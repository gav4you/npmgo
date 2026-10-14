//go:build windows

package supportscolor

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// osRelease is os.release() on Windows: libuv's uv_os_uname formats
// RtlGetVersion as major.minor.build.
func osRelease() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
