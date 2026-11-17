//go:build linux

package serverinfo

import (
	"os/exec"
	"strings"
)

func execLookPath(binary string) (string, error) {
	return exec.LookPath(binary)
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
