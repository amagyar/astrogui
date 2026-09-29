//go:build !windows

package lifecycle

import (
	"errors"
	"syscall"
)

// isCrossDevice reports whether a rename error is a cross-filesystem failure
// (EXDEV): the case where no atomic rename exists.
func isCrossDevice(err error) bool {
	return err != nil && errors.Is(err, syscall.EXDEV)
}
