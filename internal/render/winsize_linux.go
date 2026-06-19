package render

import (
	"syscall"
	"unsafe"
)

type winsize struct {
	row, col, xpix, ypix uint16
}

// termRows returns the number of rows of the terminal on fd via TIOCGWINSZ.
func termRows(fd uintptr) (int, bool) {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0, false
	}
	return int(ws.row), true
}
