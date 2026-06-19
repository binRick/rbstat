//go:build !linux

package render

// termRows has no portable implementation off Linux; callers fall back to
// printing the header once.
func termRows(fd uintptr) (int, bool) { return 0, false }
