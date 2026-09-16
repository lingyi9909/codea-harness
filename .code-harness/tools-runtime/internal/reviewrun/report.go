package reviewrun

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

//go:embed review.md.tmpl
var reviewTemplateText string

var reviewTemplate = template.Must(template.New("review.md").Parse(reviewTemplateText))

type reportView struct {
	StatusText       string
	RunID            string
	Project          string
	ReportPath       string
	CreatedAt        string
	CompletedAt      string
	Execution        string
	ReviewConclusion string
	Coverage         string
	CancelReason     string
	ResultSHA256     string
	Findings         []Finding
	PendingRisks     []string
	Gaps             []string
}

type reportMeta struct {
	SchemaVersion int    `json:"schemaVersion"`
	RunID         string `json:"runId"`
	Execution     string `json:"execution"`
	ResultSHA256  string `json:"resultSha256"`
}

const reportMetaPrefix = "<!-- codea-review-meta "

func renderReport(view reportView) ([]byte, error) {
	var buf bytes.Buffer
	if err := reviewTemplate.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("REVIEW_REPORT_RENDER_FAILED: %w", err)
	}
	out := buf.Bytes()
	if bytes.HasPrefix(out, []byte{0xef, 0xbb, 0xbf}) {
		return nil, fmt.Errorf("REVIEW_REPORT_BOM_FORBIDDEN")
	}
	return out, nil
}

func parseReportMeta(data []byte) (reportMeta, error) {
	text := strings.TrimSpace(string(data))
	idx := strings.LastIndex(text, reportMetaPrefix)
	if idx < 0 {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_MISSING")
	}
	tail := text[idx+len(reportMetaPrefix):]
	end := strings.Index(tail, " -->")
	if end < 0 {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_INVALID")
	}
	raw := tail[:end]
	var meta reportMeta
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&meta); err != nil {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_INVALID: %w", err)
	}
	if meta.SchemaVersion != SchemaVersion || meta.RunID == "" {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_INVALID")
	}
	return meta, nil
}

func bytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
