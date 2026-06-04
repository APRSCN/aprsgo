//go:build unix

package main

import (
	"os"
	"syscall"
)

// upgradeSignal returns the OS signal that triggers a live upgrade (SIGUSR2 on
// Unix-like systems).
func upgradeSignal() os.Signal { return syscall.SIGUSR2 }
