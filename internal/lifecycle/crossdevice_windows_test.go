//go:build windows

package lifecycle

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsCrossDeviceError(t *testing.T) {
	if !isCrossDevice(windows.ERROR_NOT_SAME_DEVICE) {
		t.Fatal("ERROR_NOT_SAME_DEVICE was not recognized")
	}
	if isCrossDevice(errors.New("access denied")) {
		t.Fatal("unrelated error was classified as cross-device")
	}
}
