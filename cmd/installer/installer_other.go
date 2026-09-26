//go:build !windows

package main

import (
	"fmt"
)

func runInstall(silent bool) error {
	return fmt.Errorf("the graphical installer is designed for Windows; on Linux, run 'go run main.go' directly")
}

func runUninstall(silent bool) error {
	return fmt.Errorf("the graphical installer is designed for Windows")
}
