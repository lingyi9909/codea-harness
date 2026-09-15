package analysis

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Use the existing Runtime-owned authority key outside the workspace with a
// distinct MAC domain and immutable per-run record; no Agent request exposes
// this operation. A mutable certificate field alone cannot seal a session.
type primarySessionSeal166 struct {
	RepositorySHA256  string `json:"repositorySha256"`
	CertificateSHA256 string `json:"certificateSha256"`
	MAC               string `json:"mac"`
}

func primarySessionRecord166(root, runID string) (string, string, error) {
	legacy, repo, err := chainMaintenanceAuthorityRecordPath153(root, runID)
	if err != nil {
		return "", "", err
	}
	base := filepath.Dir(filepath.Dir(filepath.Dir(legacy)))
	return filepath.Join(base, "analysis-primary-session-v1", repo, filepath.Base(legacy)), repo, nil
}
func primarySessionSealBytes166(root string, cert Certificate, create bool) ([]byte, string, error) {
	path, repo, err := primarySessionRecord166(root, cert.RunID)
	if err != nil {
		return nil, "", err
	}
	var key []byte
	if create {
		key, err = loadOrCreateChainMaintenanceAuthorityKey153(root)
	} else {
		key, err = loadExistingChainMaintenanceAuthorityKey153(root)
	}
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(cert)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(raw)
	record := primarySessionSeal166{RepositorySHA256: repo, CertificateSHA256: hex.EncodeToString(hash[:])}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("codea-primary-session-v1\x00" + repo + "\x00" + record.CertificateSHA256))
	record.MAC = hex.EncodeToString(mac.Sum(nil))
	raw, err = json.Marshal(record)
	return append(raw, '\n'), path, err
}

// SealPrimarySessionAuthority is called only after Runtime certification has
// assembled a certificate from a Host-verified primary submission.
func SealPrimarySessionAuthority(root string, cert Certificate) error {
	if cert.SemanticSessionID == "" {
		return nil
	}
	data, path, err := primarySessionSealBytes166(root, cert, true)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, e := readChainMaintenanceAuthorityRecordBytes153(path)
		if e != nil {
			return e
		}
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("PRIMARY_SESSION_AUTHORITY_CONFLICT: run already belongs to a certified session")
	}
	if err != nil {
		return err
	}
	success := false
	defer func() {
		file.Close()
		if !success {
			os.Remove(path)
		}
	}()
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	success = true
	return nil
}
func verifyPrimarySessionAuthority166(root string, cert Certificate) error {
	path, _, err := primarySessionRecord166(root, cert.RunID)
	if err != nil {
		return err
	}
	actual, err := readChainMaintenanceAuthorityRecordBytes153(path)
	if errors.Is(err, os.ErrNotExist) && cert.SemanticSessionID == "" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("PRIMARY_SESSION_AUTHORITY_MISSING: %w", err)
	}
	expected, _, err := primarySessionSealBytes166(root, cert, false)
	if err != nil {
		return err
	}
	if !hmac.Equal(actual, expected) {
		return fmt.Errorf("PRIMARY_SESSION_AUTHORITY_MISMATCH: certified session identity changed")
	}
	return nil
}
