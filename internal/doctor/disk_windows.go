//go:build windows

package doctor

import "errors"

// statfs is not supported on Windows — disk space checks are skipped.
func statfs(path string) (total, free int64, err error) {
	return 0, 0, errors.New("disk space check not supported on Windows")
}
