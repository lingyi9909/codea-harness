// Package requestjson reads agent-authored JSON requests at Runtime ingress.
// It must not be used for Runtime-certified or signed artifacts: their exact
// bytes remain part of their authority and integrity checks.
package requestjson

import (
	"bytes"
	"fmt"
	"os"
	"unicode/utf8"
)

// ReadFile accepts one UTF-8 BOM only at byte zero, for Windows-authored
// requests. It preserves all other bytes for the existing schema validators
// and strict JSON decoders; it does not rewrite the file or repair JSON.
func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("REQUEST_JSON_INVALID_UTF8: %s: request must use UTF-8", path)
	}
	return data, nil
}
