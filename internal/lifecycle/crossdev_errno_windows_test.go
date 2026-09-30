//go:build windows

package lifecycle

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// crossDeviceErrno matches what os.Rename returns on Windows when a move
// crosses a volume boundary. syscall.EXDEV (errno 18) never occurs here —
// the kernel reports ERROR_NOT_SAME_DEVICE (errno 17) instead.
func crossDeviceErrno() syscall.Errno { return windows.ERROR_NOT_SAME_DEVICE }
