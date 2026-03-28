//go:build windows

package doctor

// checkHardlinks is a no-op on Windows — hardlink diagnostics require Unix syscalls.
func checkHardlinks(_ *Result) {}
