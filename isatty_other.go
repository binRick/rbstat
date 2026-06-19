//go:build !linux

package main

// isatty has no portable implementation off Linux; treat fd as non-tty so
// color auto-disables and the header prints once (matching non-tty behavior).
func isatty(fd uintptr) bool { return false }
