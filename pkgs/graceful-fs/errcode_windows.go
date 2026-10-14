// uvTranslateSysError is translated from libuv 1.51.0 (the version bundled
// with Node v25.9.0), src/win/error.c, which carries this notice:
//
// Copyright Joyent, Inc. and other Node contributors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.

//go:build windows

package gracefulfs

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// sysErrCode returns the code Node gives the Windows error err wraps:
// libuv's uv_translate_sys_error, named as uv_err_name names it. Go's own
// invented errno values (syscall.EINVAL and the like, at or above
// APPLICATION_ERROR) are not Windows errors and come out as "UNKNOWN", as
// libuv would translate them.
func sysErrCode(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return ""
	}
	return uvTranslateSysError(errno)
}

// uvTranslateSysError is uv_translate_sys_error: the libuv error for a Win32
// or Winsock error code, by its name ("" for 0, which is no error).
func uvTranslateSysError(sysErrno syscall.Errno) string {
	if sysErrno == 0 {
		return ""
	}

	switch sysErrno {
	case windows.WSAEACCES:
		return "EACCES"
	case windows.ERROR_ELEVATION_REQUIRED:
		return "EACCES"
	case windows.ERROR_CANT_ACCESS_FILE:
		return "EACCES"
	case windows.ERROR_ADDRESS_ALREADY_ASSOCIATED:
		return "EADDRINUSE"
	case windows.WSAEADDRINUSE:
		return "EADDRINUSE"
	case windows.WSAEADDRNOTAVAIL:
		return "EADDRNOTAVAIL"
	case windows.WSAEAFNOSUPPORT:
		return "EAFNOSUPPORT"
	case windows.WSAEWOULDBLOCK:
		return "EAGAIN"
	case windows.ERROR_NO_DATA:
		return "EAGAIN"
	case windows.WSAEALREADY:
		return "EALREADY"
	case windows.ERROR_INVALID_FLAGS:
		return "EBADF"
	case windows.ERROR_INVALID_HANDLE:
		return "EBADF"
	case windows.ERROR_LOCK_VIOLATION:
		return "EBUSY"
	case windows.ERROR_PIPE_BUSY:
		return "EBUSY"
	case windows.ERROR_SHARING_VIOLATION:
		return "EBUSY"
	case windows.ERROR_OPERATION_ABORTED:
		return "ECANCELED"
	case windows.WSAEINTR:
		return "ECANCELED"
	case windows.ERROR_NO_UNICODE_TRANSLATION:
		return "ECHARSET"
	case windows.ERROR_CONNECTION_ABORTED:
		return "ECONNABORTED"
	case windows.WSAECONNABORTED:
		return "ECONNABORTED"
	case windows.ERROR_CONNECTION_REFUSED:
		return "ECONNREFUSED"
	case windows.WSAECONNREFUSED:
		return "ECONNREFUSED"
	case windows.ERROR_NETNAME_DELETED:
		return "ECONNRESET"
	case windows.WSAECONNRESET:
		return "ECONNRESET"
	case windows.ERROR_ALREADY_EXISTS:
		return "EEXIST"
	case windows.ERROR_FILE_EXISTS:
		return "EEXIST"
	case windows.ERROR_NOACCESS:
		return "EFAULT"
	case windows.WSAEFAULT:
		return "EFAULT"
	case windows.ERROR_HOST_UNREACHABLE:
		return "EHOSTUNREACH"
	case windows.WSAEHOSTUNREACH:
		return "EHOSTUNREACH"
	case windows.ERROR_INSUFFICIENT_BUFFER:
		return "EINVAL"
	case windows.ERROR_INVALID_DATA:
		return "EINVAL"
	case windows.ERROR_INVALID_PARAMETER:
		return "EINVAL"
	case windows.ERROR_SYMLINK_NOT_SUPPORTED:
		return "EINVAL"
	case windows.WSAEINVAL:
		return "EINVAL"
	case windows.WSAEPFNOSUPPORT:
		return "EINVAL"
	case windows.ERROR_BEGINNING_OF_MEDIA:
		return "EIO"
	case windows.ERROR_BUS_RESET:
		return "EIO"
	case windows.ERROR_CRC:
		return "EIO"
	case windows.ERROR_DEVICE_DOOR_OPEN:
		return "EIO"
	case windows.ERROR_DEVICE_REQUIRES_CLEANING:
		return "EIO"
	case windows.ERROR_DISK_CORRUPT:
		return "EIO"
	case windows.ERROR_EOM_OVERFLOW:
		return "EIO"
	case windows.ERROR_FILEMARK_DETECTED:
		return "EIO"
	case windows.ERROR_GEN_FAILURE:
		return "EIO"
	case windows.ERROR_INVALID_BLOCK_LENGTH:
		return "EIO"
	case windows.ERROR_IO_DEVICE:
		return "EIO"
	case windows.ERROR_NO_DATA_DETECTED:
		return "EIO"
	case windows.ERROR_NO_SIGNAL_SENT:
		return "EIO"
	case windows.ERROR_OPEN_FAILED:
		return "EIO"
	case windows.ERROR_SETMARK_DETECTED:
		return "EIO"
	case windows.ERROR_SIGNAL_REFUSED:
		return "EIO"
	case windows.WSAEISCONN:
		return "EISCONN"
	case windows.ERROR_CANT_RESOLVE_FILENAME:
		return "ELOOP"
	case windows.ERROR_TOO_MANY_OPEN_FILES:
		return "EMFILE"
	case windows.WSAEMFILE:
		return "EMFILE"
	case windows.WSAEMSGSIZE:
		return "EMSGSIZE"
	case windows.ERROR_BUFFER_OVERFLOW:
		return "ENAMETOOLONG"
	case windows.ERROR_FILENAME_EXCED_RANGE:
		return "ENAMETOOLONG"
	case windows.ERROR_NETWORK_UNREACHABLE:
		return "ENETUNREACH"
	case windows.WSAENETUNREACH:
		return "ENETUNREACH"
	case windows.WSAENOBUFS:
		return "ENOBUFS"
	case windows.ERROR_BAD_PATHNAME:
		return "ENOENT"
	case windows.ERROR_DIRECTORY:
		return "ENOENT"
	case windows.ERROR_ENVVAR_NOT_FOUND:
		return "ENOENT"
	case windows.ERROR_FILE_NOT_FOUND:
		return "ENOENT"
	case windows.ERROR_INVALID_NAME:
		return "ENOENT"
	case windows.ERROR_INVALID_DRIVE:
		return "ENOENT"
	case windows.ERROR_INVALID_REPARSE_DATA:
		return "ENOENT"
	case windows.ERROR_MOD_NOT_FOUND:
		return "ENOENT"
	case windows.ERROR_PATH_NOT_FOUND:
		return "ENOENT"
	case windows.WSAHOST_NOT_FOUND:
		return "ENOENT"
	case windows.WSANO_DATA:
		return "ENOENT"
	case windows.ERROR_NOT_ENOUGH_MEMORY:
		return "ENOMEM"
	case windows.ERROR_OUTOFMEMORY:
		return "ENOMEM"
	case windows.ERROR_CANNOT_MAKE:
		return "ENOSPC"
	case windows.ERROR_DISK_FULL:
		return "ENOSPC"
	case windows.ERROR_EA_TABLE_FULL:
		return "ENOSPC"
	case windows.ERROR_END_OF_MEDIA:
		return "ENOSPC"
	case windows.ERROR_HANDLE_DISK_FULL:
		return "ENOSPC"
	case windows.ERROR_NOT_CONNECTED:
		return "ENOTCONN"
	case windows.WSAENOTCONN:
		return "ENOTCONN"
	case windows.ERROR_DIR_NOT_EMPTY:
		return "ENOTEMPTY"
	case windows.WSAENOTSOCK:
		return "ENOTSOCK"
	case windows.ERROR_NOT_SUPPORTED:
		return "ENOTSUP"
	case windows.ERROR_BROKEN_PIPE:
		return "EOF"
	case windows.ERROR_ACCESS_DENIED:
		return "EPERM"
	case windows.ERROR_PRIVILEGE_NOT_HELD:
		return "EPERM"
	case windows.ERROR_BAD_PIPE:
		return "EPIPE"
	case windows.ERROR_PIPE_NOT_CONNECTED:
		return "EPIPE"
	case windows.WSAESHUTDOWN:
		return "EPIPE"
	case windows.WSAEPROTONOSUPPORT:
		return "EPROTONOSUPPORT"
	case windows.ERROR_WRITE_PROTECT:
		return "EROFS"
	case windows.ERROR_SEM_TIMEOUT:
		return "ETIMEDOUT"
	case windows.WSAETIMEDOUT:
		return "ETIMEDOUT"
	case windows.ERROR_NOT_SAME_DEVICE:
		return "EXDEV"
	case windows.ERROR_INVALID_FUNCTION:
		return "EISDIR"
	case windows.ERROR_META_EXPANSION_TOO_LONG:
		return "E2BIG"
	case windows.WSAESOCKTNOSUPPORT:
		return "ESOCKTNOSUPPORT"
	case windows.ERROR_BAD_EXE_FORMAT:
		return "EFTYPE"
	default:
		return "UNKNOWN"
	}
}
