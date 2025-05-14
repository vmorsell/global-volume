//go:build darwin
// +build darwin

package volume

import (
	"fmt"
	"os/exec"
)

// getSystemVolume returns the current output volume (0-100) on macOS.
func getSystemVolume() (int, error) {
	out, err := exec.Command("osascript", "-e", "output volume of (get volume settings)").Output()
	if err != nil {
		return 0, err
	}
	var vol int
	_, err = fmt.Sscanf(string(out), "%d", &vol)
	return vol, err
}
