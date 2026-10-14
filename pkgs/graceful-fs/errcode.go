package gracefulfs

import "errors"

// coder is an error that reports its Node error code (err.code).
type coder interface{ Code() string }

// errCode returns err.code as graceful-fs reads it: the code the error, or
// an error it wraps, reports through a Code() string method; otherwise the
// libuv name of the system error it wraps (see sysErrCode); otherwise "".
func errCode(err error) string {
	var c coder
	if errors.As(err, &c) {
		return c.Code()
	}
	return sysErrCode(err)
}
