package analysis

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	chainMaintenanceAuthorityVersion153 = 1
	chainMaintenanceAuthorityKeySize153 = 32
)

type chainMaintenanceAuthorityPayload153 struct {
	Version          int    `json:"version"`
	RepositorySHA256 string `json:"repositorySha256"`
	RunID            string `json:"runId"`
	ChangeSetSHA256  string `json:"changeSetSha256"`
	AnalysisSHA256   string `json:"analysisSha256"`
	Intent           Intent `json:"intent"`
}

type chainMaintenanceAuthorityRecord153 struct {
	chainMaintenanceAuthorityPayload153
	MAC string `json:"mac"`
}

// sealChainMaintenanceAuthority153 records privileged CHAIN_MAINTENANCE intent
// outside the workspace, protected by a Runtime-owned HMAC key. Ordinary FULL,
// LIST, and TARGETED certifications do not create edit authority.
func sealChainMaintenanceAuthority153(root string, cert Certificate) error {
	if cert.Intent == nil || strings.ToUpper(strings.TrimSpace(cert.Intent.Mode)) != "CHAIN_MAINTENANCE" {
		return nil
	}
	intent, err := normalizedChainMaintenanceIntent153(cert.Intent)
	if err != nil {
		return err
	}
	payload, recordPath, err := chainMaintenanceAuthorityPayloadForCertificate153(root, cert, intent)
	if err != nil {
		return err
	}
	key, err := loadOrCreateChainMaintenanceAuthorityKey153(root)
	if err != nil {
		return err
	}
	record := chainMaintenanceAuthorityRecord153{chainMaintenanceAuthorityPayload153: payload}
	record.MAC, err = chainMaintenanceAuthorityMAC153(key, payload)
	if err != nil {
		return err
	}
	data, err := canonicalChainMaintenanceAuthorityRecord153(record)
	if err != nil {
		return err
	}

	if existing, readErr := readChainMaintenanceAuthorityRecordBytes153(recordPath); readErr == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_CONFLICT: sealed Runtime authority differs")
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}

	if err := os.MkdirAll(filepath.Dir(recordPath), 0o700); err != nil {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_DIR_FAILED: %w", err)
	}
	_ = os.Chmod(filepath.Dir(recordPath), 0o700)
	f, err := os.OpenFile(recordPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := readChainMaintenanceAuthorityRecordBytes153(recordPath)
		if readErr != nil {
			return readErr
		}
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_CONFLICT: sealed Runtime authority differs")
	}
	if err != nil {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_CREATE_FAILED: %w", err)
	}
	clean := true
	defer func() {
		_ = f.Close()
		if clean {
			_ = os.Remove(recordPath)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_WRITE_FAILED: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_WRITE_FAILED: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_WRITE_FAILED: %w", err)
	}
	clean = false
	return nil
}

func verifyChainMaintenanceAuthority153(root string, cert Certificate) error {
	if cert.Intent == nil || strings.ToUpper(strings.TrimSpace(cert.Intent.Mode)) != "CHAIN_MAINTENANCE" {
		return nil
	}
	intent, err := normalizedChainMaintenanceIntent153(cert.Intent)
	if err != nil {
		return err
	}
	recordPath, repoDigest, err := chainMaintenanceAuthorityRecordPath153(root, cert.RunID)
	if err != nil {
		return err
	}
	data, err := readChainMaintenanceAuthorityRecordBytes153(recordPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_MISSING")
	}
	if err != nil {
		return err
	}
	var record chainMaintenanceAuthorityRecord153
	if err := decodeStrictChainMaintenanceAuthority153(data, &record); err != nil {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: invalid sealed record: %w", err)
	}
	canonical, err := canonicalChainMaintenanceAuthorityRecord153(record)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, canonical) {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: sealed record bytes changed")
	}
	if record.Version != chainMaintenanceAuthorityVersion153 ||
		record.RepositorySHA256 != repoDigest ||
		record.RunID != cert.RunID ||
		record.ChangeSetSHA256 != cert.ChangeSetSHA256 ||
		record.AnalysisSHA256 != cert.AnalysisSHA256 ||
		record.Intent != intent {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_MISMATCH: sealed Runtime authority identity differs")
	}
	key, err := loadExistingChainMaintenanceAuthorityKey153(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_KEY_MISSING")
		}
		return err
	}
	wantMAC, err := chainMaintenanceAuthorityMAC153(key, record.chainMaintenanceAuthorityPayload153)
	if err != nil {
		return err
	}
	gotBytes, err := hex.DecodeString(strings.TrimSpace(record.MAC))
	if err != nil || len(gotBytes) != sha256.Size {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: sealed record MAC invalid")
	}
	wantBytes, _ := hex.DecodeString(wantMAC)
	if !hmac.Equal(gotBytes, wantBytes) {
		return fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: sealed record MAC mismatch")
	}
	return nil
}

func normalizedChainMaintenanceIntent153(intent *Intent) (Intent, error) {
	if intent == nil {
		return Intent{}, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_INTENT_INVALID: missing intent")
	}
	normalized := Intent{
		Mode:   strings.ToUpper(strings.TrimSpace(intent.Mode)),
		Target: strings.TrimSpace(intent.Target),
	}
	if normalized.Mode != "CHAIN_MAINTENANCE" || normalized.Target == "" {
		return Intent{}, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_INTENT_INVALID: exact target is required")
	}
	return normalized, nil
}

func chainMaintenanceAuthorityPayloadForCertificate153(root string, cert Certificate, intent Intent) (chainMaintenanceAuthorityPayload153, string, error) {
	recordPath, repoDigest, err := chainMaintenanceAuthorityRecordPath153(root, cert.RunID)
	if err != nil {
		return chainMaintenanceAuthorityPayload153{}, "", err
	}
	if strings.TrimSpace(cert.RunID) == "" || len(strings.TrimSpace(cert.ChangeSetSHA256)) != 64 || len(strings.TrimSpace(cert.AnalysisSHA256)) != 64 {
		return chainMaintenanceAuthorityPayload153{}, "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_IDENTITY_INVALID")
	}
	return chainMaintenanceAuthorityPayload153{
		Version:          chainMaintenanceAuthorityVersion153,
		RepositorySHA256: repoDigest,
		RunID:            cert.RunID,
		ChangeSetSHA256:  cert.ChangeSetSHA256,
		AnalysisSHA256:   cert.AnalysisSHA256,
		Intent:           intent,
	}, recordPath, nil
}

func chainMaintenanceAuthorityRecordPath153(root, runID string) (string, string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_ROOT_FAILED: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)
	repoSum := sha256.Sum256([]byte(filepath.ToSlash(rootAbs)))
	repoDigest := hex.EncodeToString(repoSum[:])
	runSum := sha256.Sum256([]byte(strings.TrimSpace(runID)))
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	base := filepath.Join(configDir, "codea-harness", "runtime-authority", "analysis-chain-maintenance-v1")
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	if pathWithinWorkspace153(rootAbs, baseAbs) {
		return "", "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_HOME_INVALID: authority store must be outside workspace")
	}
	return filepath.Join(baseAbs, repoDigest, hex.EncodeToString(runSum[:])+".json"), repoDigest, nil
}

func chainMaintenanceAuthorityKeyPath153(root string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_ROOT_FAILED: %w", err)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	keyPath, err := filepath.Abs(filepath.Join(configDir, "codea-harness", "runtime-authority", "analysis-chain-maintenance-v1.key"))
	if err != nil {
		return "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	if pathWithinWorkspace153(rootAbs, keyPath) {
		return "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_HOME_INVALID: authority key must be outside workspace")
	}
	return keyPath, nil
}

func pathWithinWorkspace153(rootAbs, candidateAbs string) bool {
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func loadOrCreateChainMaintenanceAuthorityKey153(root string) ([]byte, error) {
	if key, err := loadExistingChainMaintenanceAuthorityKey153(root); err == nil {
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	path, err := chainMaintenanceAuthorityKeyPath153(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_DIR_FAILED: %w", err)
	}
	_ = os.Chmod(filepath.Dir(path), 0o700)
	key := make([]byte, chainMaintenanceAuthorityKeySize153)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_RANDOM_FAILED: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadExistingChainMaintenanceAuthorityKey153(root)
	}
	if err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_KEY_CREATE_FAILED: %w", err)
	}
	clean := true
	defer func() {
		_ = f.Close()
		if clean {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(key); err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_KEY_WRITE_FAILED: %w", err)
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_KEY_WRITE_FAILED: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_KEY_WRITE_FAILED: %w", err)
	}
	clean = false
	return key, nil
}

func loadExistingChainMaintenanceAuthorityKey153(root string) ([]byte, error) {
	path, err := chainMaintenanceAuthorityKeyPath153(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: authority key type invalid")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: authority key permissions are too broad")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_KEY_READ_FAILED: %w", err)
	}
	if len(key) != chainMaintenanceAuthorityKeySize153 {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: authority key length invalid")
	}
	return key, nil
}

func readChainMaintenanceAuthorityRecordBytes153(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: sealed record type invalid")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_TAMPERED: sealed record permissions are too broad")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_READ_FAILED: %w", err)
	}
	return data, nil
}

func chainMaintenanceAuthorityMAC153(key []byte, payload chainMaintenanceAuthorityPayload153) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_ENCODE_FAILED: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func canonicalChainMaintenanceAuthorityRecord153(record chainMaintenanceAuthorityRecord153) ([]byte, error) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("CHAIN_MAINTENANCE_AUTHORITY_ENCODE_FAILED: %w", err)
	}
	return append(data, '\n'), nil
}

func decodeStrictChainMaintenanceAuthority153(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}
