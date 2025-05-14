//go:build windows
// +build windows

package volume

import "fmt"

// getSystemVolume returns the current output volume (0-100) on Windows.
// This is a stub. You can implement using nircmd or Windows API.
func getSystemVolume() (int, error) {
	return 0, fmt.Errorf("getSystemVolume not implemented for Windows")
}
