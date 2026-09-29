//go:build windows

package lifecycle

import (
	"errors"
	"strings"
	"syscall"
)

// isCrossDevice reports whether a rename error means the source and
// destination are on different volumes, where no atomic rename exists.
func isCrossDevice(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ERROR_NOT_SAME_DEVICE) ||
		strings.Contains(err.Error(), "not the same")
}
