//go:build darwin

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct {
	Rows, Cols, Xpixel, Ypixel uint16
}

func terminalSize(f *os.File) (int, int) {
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(0x40087468), uintptr(unsafe.Pointer(ws)))
	if errno == 0 && ws.Cols > 0 && ws.Rows > 0 {
		return int(ws.Cols), int(ws.Rows)
	}
	return 100, 30
}
