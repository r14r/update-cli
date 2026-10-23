//go:build !linux && !darwin

package ui

import "os"

func terminalSize(_ *os.File) (int, int) { return 100, 30 }
