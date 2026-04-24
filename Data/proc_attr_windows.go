//go:build windows

package data

import "syscall"

// HiddenProcAttr returns a SysProcAttr that hides the console window on Windows.
func HiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
