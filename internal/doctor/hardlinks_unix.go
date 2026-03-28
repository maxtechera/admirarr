//go:build !windows

package doctor

import (
	"os"
	"syscall"
)

// sameFilesystem reports whether two FileInfo entries refer to paths on the same
// device (prerequisite for hardlinks).  Returns (same, ok) where ok=false means
// the check is not supported on this platform.
func sameFilesystem(a, b os.FileInfo) (same bool, ok bool) {
	sa, aOk := a.Sys().(*syscall.Stat_t)
	sb, bOk := b.Sys().(*syscall.Stat_t)
	if !aOk || !bOk {
		return false, false
	}
	return sa.Dev == sb.Dev, true
}
