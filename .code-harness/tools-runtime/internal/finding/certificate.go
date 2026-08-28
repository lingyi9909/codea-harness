package finding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codea-harness-tools/internal/schema"
)

func WriteCertified(repoRoot string, set CertifiedSet, cert Certificate) error {
	root := filepath.Clean(repoRoot)
	if !findingRunID160.MatchString(set.RunID) || set.RunID != cert.RunID {
		return findingError160("CERTIFIED_FINDINGS_IDENTITY_MISMATCH", "set/certificate runId mismatch")
	}
	if set.ChangeSetSHA256 != cert.ChangeSetSHA256 || set.ChangeAnalysisSHA256 != cert.ChangeAnalysisSHA256 || set.ReviewUnitsSHA256 != cert.ReviewUnitsSHA256 || set.RuleDispatchSHA256 != cert.RuleDispatchSHA256 || set.FindingProposalsSHA256 != cert.FindingProposalsSHA256 {
		return findingError160("CERTIFIED_FINDINGS_IDENTITY_MISMATCH", "certificate authority hashes do not match Certified Findings")
	}
	if strings.TrimSpace(set.HarnessVersion) == "" || !validSHA160(set.ChangeSetSHA256) || !validSHA160(set.ChangeAnalysisSHA256) || !validSHA160(set.ReviewUnitsSHA256) || !validSHA160(set.RuleDispatchSHA256) || !validSHA160(set.FindingProposalsSHA256) {
		return findingError160("CERTIFIED_FINDINGS_INVALID", "invalid Certified Findings identity")
	}
	mode := strings.ToUpper(strings.TrimSpace(cert.Mode))
	if mode != "FULL" && mode != "TARGETED" {
		return findingError160("CERTIFIED_FINDINGS_INVALID", "invalid certificate mode %q", cert.Mode)
	}
	cert.Mode = mode
	if cert.ScopeSHA256 != "" && !validSHA160(cert.ScopeSHA256) {
		return findingError160("CERTIFIED_FINDINGS_INVALID", "invalid scopeSha256")
	}
	if set.Findings == nil {
		set.Findings = []CertifiedFinding{}
	}
	unsigned, err := canonicalCertifiedSet160(set, false)
	if err != nil {
		return findingError160("CERTIFIED_FINDINGS_ENCODE_FAILED", "%v", err)
	}
	computedSetSHA := hashFindingBytes160(unsigned)
	if set.SHA256 != "" && set.SHA256 != computedSetSHA {
		return findingError160("CERTIFIED_FINDINGS_HASH_MISMATCH", "set sha256 mismatch")
	}
	set.SHA256 = computedSetSHA
	setBytes, err := canonicalCertifiedSet160(set, true)
	if err != nil {
		return findingError160("CERTIFIED_FINDINGS_ENCODE_FAILED", "%v", err)
	}
	computedArtifactSHA := hashFindingBytes160(setBytes)
	if cert.CertifiedFindingsSHA256 != "" && cert.CertifiedFindingsSHA256 != computedArtifactSHA {
		return findingError160("CERTIFIED_FINDINGS_HASH_MISMATCH", "certificate artifact hash mismatch")
	}
	cert.CertifiedFindingsSHA256 = computedArtifactSHA
	certBytes, err := canonicalCertificate160(cert)
	if err != nil {
		return findingError160("CERTIFIED_FINDINGS_CERT_ENCODE_FAILED", "%v", err)
	}
	if err := validateFindingArtifactSchema160(root, "certified-findings.schema.json", setBytes); err != nil {
		return err
	}
	if err := validateFindingArtifactSchema160(root, "certified-findings-cert.schema.json", certBytes); err != nil {
		return err
	}
	analysisDir := filepath.Join(root, ".code-harness", "runs", set.RunID, "analysis")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil {
		return findingError160("CERTIFIED_FINDINGS_WRITE_FAILED", "create analysis dir: %v", err)
	}
	if err := atomicFindingWrite160(filepath.Join(analysisDir, "certified-findings.json"), setBytes); err != nil {
		return err
	}
	// Publish certificate last so a consumer never sees a new certificate before its set.
	if err := atomicFindingWrite160(filepath.Join(analysisDir, "certified-findings.cert.json"), certBytes); err != nil {
		return err
	}
	return nil
}

func LoadCertified(repoRoot, runID string) (CertifiedSet, error) {
	set, _, err := LoadCertifiedWithCertificate(repoRoot, runID)
	return set, err
}

func LoadCertifiedWithCertificate(repoRoot, runID string) (CertifiedSet, Certificate, error) {
	root := filepath.Clean(repoRoot)
	runID = strings.TrimSpace(runID)
	if !findingRunID160.MatchString(runID) {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_RUN_ID_INVALID", "%q", runID)
	}
	analysisDir := filepath.Join(root, ".code-harness", "runs", runID, "analysis")
	setPath := filepath.Join(analysisDir, "certified-findings.json")
	certPath := filepath.Join(analysisDir, "certified-findings.cert.json")
	setBytes, err := os.ReadFile(setPath)
	if err != nil {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_READ_FAILED", "%v", err)
	}
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_CERT_READ_FAILED", "%v", err)
	}
	if err := validateFindingArtifactSchema160(root, "certified-findings.schema.json", setBytes); err != nil {
		return CertifiedSet{}, Certificate{}, err
	}
	if err := validateFindingArtifactSchema160(root, "certified-findings-cert.schema.json", certBytes); err != nil {
		return CertifiedSet{}, Certificate{}, err
	}
	var set CertifiedSet
	if err := strictFindingDecode160(setBytes, &set); err != nil {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_DECODE_FAILED", "%v", err)
	}
	var cert Certificate
	if err := strictFindingDecode160(certBytes, &cert); err != nil {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_CERT_DECODE_FAILED", "%v", err)
	}
	canonicalSet, err := canonicalCertifiedSet160(set, true)
	if err != nil || !bytes.Equal(setBytes, canonicalSet) {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_BYTES_NOT_CANONICAL", "artifact bytes are not canonical")
	}
	canonicalCert, err := canonicalCertificate160(cert)
	if err != nil || !bytes.Equal(certBytes, canonicalCert) {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_CERT_BYTES_NOT_CANONICAL", "certificate bytes are not canonical")
	}
	unsigned, err := canonicalCertifiedSet160(set, false)
	if err != nil || hashFindingBytes160(unsigned) != set.SHA256 {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_HASH_MISMATCH", "internal sha256 mismatch")
	}
	if cert.RunID != runID || set.RunID != runID || cert.CertifiedFindingsSHA256 != hashFindingBytes160(setBytes) {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_IDENTITY_MISMATCH", "run/artifact identity mismatch")
	}
	if set.ChangeSetSHA256 != cert.ChangeSetSHA256 || set.ChangeAnalysisSHA256 != cert.ChangeAnalysisSHA256 || set.ReviewUnitsSHA256 != cert.ReviewUnitsSHA256 || set.RuleDispatchSHA256 != cert.RuleDispatchSHA256 || set.FindingProposalsSHA256 != cert.FindingProposalsSHA256 {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_IDENTITY_MISMATCH", "certificate authority hashes differ from set")
	}
	current := []struct {
		name string
		path string
		want string
	}{
		{"CHANGE_ANALYSIS", filepath.Join(analysisDir, "change-analysis.json"), cert.ChangeAnalysisSHA256},
		{"REVIEW_UNITS", filepath.Join(analysisDir, "review-units.json"), cert.ReviewUnitsSHA256},
		{"RULE_DISPATCH", filepath.Join(analysisDir, "rule-dispatch.json"), cert.RuleDispatchSHA256},
		{"FINDING_PROPOSALS", filepath.Join(root, ".code-harness", "runs", runID, "requests", "finding-proposals.json"), cert.FindingProposalsSHA256},
	}
	for _, authority := range current {
		data, err := os.ReadFile(authority.path)
		if err != nil {
			return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_STALE", "CHANGED_%s_HASH_MISMATCH: read authority: %v", authority.name, err)
		}
		if hashFindingBytes160(data) != authority.want {
			return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_STALE", "CHANGED_%s_HASH_MISMATCH: authority bytes changed", authority.name)
		}
	}

	// Re-enter the existing Runtime authority instead of trusting unchanged run artifacts.
	// This revalidates Certified ChangeAnalysis against the current Working Tree/Change Set,
	// ReviewUnit/ReviewScope identity, current Runtime VERSION, and current RuleDispatch/catalog.
	authority, err := LoadVerifyContext(root, runID, "")
	if err != nil {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_STALE", "upstream Runtime authority stale: %v", err)
	}
	mode := strings.ToUpper(strings.TrimSpace(cert.Mode))
	if string(authority.units.Mode) != mode || authority.units.RunID != runID || authority.units.ChangeSetSHA256 != cert.ChangeSetSHA256 || authority.units.ChangeAnalysisSHA256 != cert.ChangeAnalysisSHA256 || authority.units.HarnessVersion != set.HarnessVersion {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_STALE", "Runtime authority identity differs from Certified Findings")
	}
	if authority.units.ReviewScopeSHA256 != strings.TrimSpace(cert.ScopeSHA256) {
		return CertifiedSet{}, Certificate{}, findingError160("CERTIFIED_FINDINGS_STALE", "Runtime ReviewScope identity differs from certificate")
	}
	return set, cert, nil
}

func validateFindingArtifactSchema160(root, name string, data []byte) error {
	schemaBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", name))
	if err != nil {
		return findingError160("CERTIFIED_FINDINGS_SCHEMA_READ_FAILED", "%s: %v", name, err)
	}
	if err := schema.ValidateJSON(schemaBytes, data); err != nil {
		return findingError160("CERTIFIED_FINDINGS_SCHEMA_INVALID", "%s: %v", name, err)
	}
	return nil
}

func strictFindingDecode160(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("multiple JSON values")
	}
	return nil
}

func atomicFindingWrite160(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".certified-findings-*.tmp")
	if err != nil {
		return findingError160("CERTIFIED_FINDINGS_WRITE_FAILED", "%v", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return findingError160("CERTIFIED_FINDINGS_WRITE_FAILED", "%v", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return findingError160("CERTIFIED_FINDINGS_WRITE_FAILED", "%v", err)
	}
	if err := tmp.Close(); err != nil {
		return findingError160("CERTIFIED_FINDINGS_WRITE_FAILED", "%v", err)
	}
	if err := os.Rename(name, target); err != nil {
		return findingError160("CERTIFIED_FINDINGS_WRITE_FAILED", "%v", err)
	}
	return nil
}
