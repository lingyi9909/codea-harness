//go:build windows

package main

import (
	"io"
	"os"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// writeWindowsConsoleError180 uses UTF-16 WriteConsoleW for an attached native
// Windows terminal. It deliberately does not change the process or parent
// console codepage. Redirected pipes use ordinary UTF-8 instead.
func writeWindowsConsoleError180(w io.Writer, message string) bool {
	file, ok := w.(*os.File)
	if !ok || file != os.Stderr {
		return false
	}
	handle := windows.Handle(file.Fd())
	var mode uint32
	if windows.GetConsoleMode(handle, &mode) != nil {
		return false
	}
	encoded := utf16.Encode([]rune(message + "\n"))
	if len(encoded) == 0 {
		return true
	}
	var written uint32
	if err := windows.WriteConsole(handle, &encoded[0], uint32(len(encoded)), &written, nil); err != nil {
		return false
	}
	return written == uint32(len(encoded))
}
