//go:build !windows

package lifecycle

import "syscall"

// crossDeviceErrno is the errno os.Rename surfaces on this platform when a
// move crosses a filesystem boundary.
func crossDeviceErrno() syscall.Errno { return syscall.EXDEV }
