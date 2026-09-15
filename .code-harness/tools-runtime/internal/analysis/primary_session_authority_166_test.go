package analysis

import (
	"strings"
	"testing"
)

func TestPrimarySessionCertificateCannotBeRebound166(t *testing.T) {
	root := t.TempDir()
	cert := Certificate{RunID: "review-166", RuntimeVersion: "1.6.6", AnalysisSHA256: strings.Repeat("a", 64), ChangeSetSHA256: strings.Repeat("b", 64), SemanticSessionID: "primary-A"}
	if err := SealPrimarySessionAuthority(root, cert); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrimarySessionAuthority166(root, cert); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{"primary-B", ""} {
		forged := cert
		forged.SemanticSessionID = replacement
		if err := verifyPrimarySessionAuthority166(root, forged); err == nil {
			t.Fatal("mutable owner accepted")
		}
		if replacement != "" {
			if err := SealPrimarySessionAuthority(root, forged); err == nil {
				t.Fatal("run owner overwritten")
			}
		}
	}
}
