//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

// isatty reports whether fd is a terminal (TCGETS succeeds only on a tty).
func isatty(fd uintptr) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&t)))
	return errno == 0
}
