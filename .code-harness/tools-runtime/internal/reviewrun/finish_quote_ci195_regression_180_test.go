package reviewrun

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

// CI #195: a real model dedented a Java multiline quote, then corrected only
// startLine/endLine. Invalid evidence MUST remain invalid; exact source evidence
// (including a literal single-line substring) MUST succeed.
func Test180QuoteCI195RejectsDedentAndAcceptsVerbatimLine(t *testing.T) {
    root := t.TempDir()
    path := "src/main/java/com/example/OrderServiceImpl.java"
    source := []byte("package com.example;\n" +
        "public class OrderServiceImpl {\n" +
        "    public boolean updateStatus(String tenantId, long id, String status) {\n" +
        "        if (status == null) { return false; }\n" +
        "        String expectedStatus;\n" +
        "        switch (status) {\n" +
        "            case \"PAID\" -> expectedStatus = \"PENDING\";\n" +
        "        }\n" +
        "        int updated = mapper.updateStatus(tenantId, id, status, expectedStatus);\n" +
        "        return updated == 1;\n" +
        "    }\n" +
        "}\n")
    abs := filepath.Join(root, filepath.FromSlash(path))
    if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(abs, source, 0o600); err != nil {
        t.Fatal(err)
    }
    sha := bytesSHA256(source)
    declared := ReadRef{Path: path, SHA256: sha, StartLine: 1, EndLine: 12}
    check := func(t *testing.T, ref ReadRef, quote string, wantValid bool) {
        t.Helper()
        finding := Finding{
            ID: "F1", Severity: "CRITICAL",
            Problem: "unsafe update", Impact: "tenant data exposed",
            Recommendation: "scope update by tenant", Verification: "verify SQL",
            Evidence: []Evidence{{Ref: ref, Quote: quote}},
        }
        err := validateFinishRequest(root, FinishRequest{
            Reads: []ReadRef{declared}, Findings: []Finding{finding},
        })
        if wantValid {
            if err != nil {
                t.Fatalf("verbatim source quote rejected: %v", err)
            }
        } else if err == nil || !strings.Contains(err.Error(), "REVIEW_FINISH_EVIDENCE_QUOTE_MISMATCH") {
            t.Fatalf("invalid quote accepted (or misclassified): %v", err)
        }
    }
    t.Run("one-line-exact-substring", func(t *testing.T) {
        ref := declared
        ref.StartLine, ref.EndLine = 9, 9
        check(t, ref, "int updated = mapper.updateStatus(tenantId, id, status, expectedStatus);", true)
    })
    t.Run("multiline-verbatim", func(t *testing.T) {
        ref := declared
        ref.StartLine, ref.EndLine = 4, 6
        check(t, ref, "        if (status == null) { return false; }\n        String expectedStatus;\n        switch (status) {", true)
    })
    t.Run("dedented-after-ref-repair", func(t *testing.T) {
        ref := declared
        ref.StartLine, ref.EndLine = 4, 6
        check(t, ref, "if (status == null) { return false; }\nString expectedStatus;\nswitch (status) {", false)
    })
    t.Run("compressed-multiline-fake-quote", func(t *testing.T) {
        ref := declared
        ref.StartLine, ref.EndLine = 4, 6
        check(t, ref, "if (status == null) { return false; } String expectedStatus; switch (status) {", false)
    })
    t.Run("correct-quote-wrong-line", func(t *testing.T) {
        ref := declared
        ref.StartLine, ref.EndLine = 8, 8
        check(t, ref, "int updated = mapper.updateStatus(tenantId, id, status, expectedStatus);", false)
    })
}
