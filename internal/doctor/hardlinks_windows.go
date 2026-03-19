//go:build windows

package doctor

import "os"

// sameFilesystem is not supported on Windows — device ID checks require
// platform-specific syscall types unavailable on this OS.
func sameFilesystem(_, _ os.FileInfo) (same bool, ok bool) {
	return false, false
}
