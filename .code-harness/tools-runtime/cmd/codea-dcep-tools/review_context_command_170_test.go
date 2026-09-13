package main

import (
	"strings"
	"testing"
)

func Test170ReviewContextRequestAcceptsMinimalRuntimeOwnedInput(t *testing.T) {
	req, err := decodeReviewContextRequest170("review-case", []byte(`{"runId":"review-case","phase":"DISCOVERY"}`))
	if err != nil { t.Fatal(err) }
	if req.RunID != "review-case" || req.Phase != "DISCOVERY" { t.Fatalf("request=%+v", req) }
}

func Test170ReviewContextRequestRejectsAgentSuppliedAuthority(t *testing.T) {
	for _, field := range []string{"roots","files","baseRef","snapshot","budget","seeds"} {
		raw := []byte(`{"runId":"review-case","phase":"DISCOVERY","`+field+`":[]}`)
		if field == "baseRef" || field == "snapshot" { raw = []byte(`{"runId":"review-case","phase":"DISCOVERY","`+field+`":"evil"}`) }
		if _, err := decodeReviewContextRequest170("review-case", raw); err == nil { t.Fatalf("agent field %s accepted", field) }
	}
}

func Test170ReviewContextRequestRejectsRunMismatch(t *testing.T) {
	_, err := decodeReviewContextRequest170("review-path", []byte(`{"runId":"review-body","phase":"DISCOVERY"}`))
	if err == nil || !strings.Contains(err.Error(), "RUN_ID_MISMATCH") { t.Fatalf("err=%v", err) }
}

func Test170ReviewContextRequestRejectsUnknownPhase(t *testing.T) {
	if _, err := decodeReviewContextRequest170("review-case", []byte(`{"runId":"review-case","phase":"EXECUTE"}`)); err == nil { t.Fatal("unknown phase accepted") }
}
