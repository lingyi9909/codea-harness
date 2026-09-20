package requestjson

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFilePreservesBytesExceptOneLeadingBOM(t *testing.T) {
	const bom = "\xef\xbb\xbf"
	for _, tc := range []struct{ name, input, want string }{
		{"plain", `{"text":"中文"}`, `{"text":"中文"}`},
		{"leading BOM", bom + `{"text":"中文"}`, `{"text":"中文"}`},
		{"whitespace", bom + " \r\n{}\t", " \r\n{}\t"},
		{"double BOM", bom + bom + `{}`, bom + `{}`},
		{"embedded BOM", `{"text":"` + bom + `"}`, `{"text":"` + bom + `"}`},
		{"nonleading BOM", " " + bom + `{}`, " " + bom + `{}`},
		{"malformed JSON", bom + `{"text":`, `{"text":`},
		{"trailing JSON", bom + `{} {}`, `{} {}`},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "request.json")
			if err := os.WriteFile(path, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			onDisk, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(onDisk, []byte(tc.input)) {
				t.Fatal("ingress modified request file bytes")
			}
		})
	}
}

func TestReadFileRejectsInvalidUTF8(t *testing.T) {
	for _, input := range []string{"\xff\xfe{\x00}\x00", "\xfe\xff\x00{\x00}", `{"text":"` + "\xff" + `"}`, "\xef\xbb", "\xef\xbb\xbf\xc0\xaf"} {
		path := filepath.Join(t.TempDir(), "request.json")
		if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		data, err := ReadFile(path)
		if err == nil || !strings.Contains(err.Error(), "REQUEST_JSON_INVALID_UTF8") || data != nil {
			t.Fatalf("invalid UTF-8 %q: got %q, %v", input, data, err)
		}
	}
}

func TestReadFilePreservesFilesystemErrors(t *testing.T) {
	_, err := ReadFile(filepath.Join(t.TempDir(), "missing.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error lost: %v", err)
	}
}
