//go:build !android

package data

// HasAllFilesAccess is always true off Android, where file access is governed
// by the ordinary file system permissions.
func HasAllFilesAccess() bool { return true }

// RequestAllFilesAccess does nothing off Android.
func RequestAllFilesAccess(appID string) error { return nil }
