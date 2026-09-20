package analysis

import "encoding/json"

// isLegacyCertificateWireE737 distinguishes the real pre-hotfix 1.6.2 wire
// format from the retained legacy Certify path that still exists in the current
// Runtime. e737 certificates predate includeWorkingTree on Certificate; current
// retained certificates encode the field even though they do not carry the
// canonical Snapshot identity fields.
func isLegacyCertificateWireE737(data []byte, cert Certificate) (bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return false, err
	}
	_, includeWorkingTreePresent := fields["includeWorkingTree"]
	canonicalIdentityPresent := cert.ResolvedBaseCommit != "" || cert.MergeBase != "" || cert.CurrentBranch != "" || cert.SnapshotSHA256 != ""
	return !includeWorkingTreePresent && !canonicalIdentityPresent, nil
}

// marshalCertificateForByteVerification162 keeps byte-for-byte canonical
// verification strict while reproducing the exact e737 field set only when the
// input bytes are positively identified as that pre-hotfix wire format.
func marshalCertificateForByteVerification162(cert Certificate, legacyE737 bool) ([]byte, error) {
	if !legacyE737 {
		return json.MarshalIndent(cert, "", "  ")
	}
	type certificateWireE737 struct {
		RunID                     string  `json:"runId"`
		RuntimeVersion            string  `json:"runtimeVersion"`
		AnalysisSHA256            string  `json:"analysisSha256"`
		ChangeSetSHA256           string  `json:"changeSetSha256"`
		EntrypointInventorySHA256 string  `json:"entrypointInventorySha256"`
		BaseRef                   string  `json:"baseRef"`
		Head                      string  `json:"head"`
		Intent                    *Intent `json:"intent,omitempty"`
	}
	return json.MarshalIndent(certificateWireE737{
		RunID: cert.RunID,
		RuntimeVersion: cert.RuntimeVersion,
		AnalysisSHA256: cert.AnalysisSHA256,
		ChangeSetSHA256: cert.ChangeSetSHA256,
		EntrypointInventorySHA256: cert.EntrypointInventorySHA256,
		BaseRef: cert.BaseRef,
		Head: cert.Head,
		Intent: cert.Intent,
	}, "", "  ")
}
