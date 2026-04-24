//go:build !windows

package data

import "syscall"

// HiddenProcAttr returns an empty SysProcAttr on non-Windows platforms.
func HiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
