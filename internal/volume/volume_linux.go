//go:build linux
// +build linux

package volume

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

// getSystemVolume returns the current output volume (0-100) on Linux using amixer.
func getSystemVolume() (int, error) {
	out, err := exec.Command("amixer", "get", "Master").Output()
	if err != nil {
		return 0, err
	}
	// Look for [XX%] in the output
	re := regexp.MustCompile(`\[(\d+)%\]`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse amixer output")
	}
	vol, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	return vol, nil
}
