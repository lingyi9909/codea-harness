//go:build !windows

package main

import "io"

// Non-Windows stderr is passed through as UTF-8.
func writeWindowsConsoleError180(io.Writer, string) bool {
	return false
}
