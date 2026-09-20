package reviewselection

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
)

const (
	reviewIntentAuthorityVersion153 = 1
	reviewIntentAuthorityKeySize153 = 32
)

type reviewIntentAuthorityPayload153 struct {
	Version          int                    `json:"version"`
	RepositorySHA256 string                 `json:"repositorySha256"`
	RunID            string                 `json:"runId"`
	ChangeSetSHA256  string                 `json:"changeSetSha256"`
	AnalysisSHA256   string                 `json:"analysisSha256"`
	Intent           analysisruntime.Intent `json:"intent"`
}

type reviewIntentAuthorityRecord153 struct {
	reviewIntentAuthorityPayload153
	MAC string `json:"mac"`
}

func sealReviewIntentAuthority153(root string, origin OptionsOrigin) error {
	payload, recordPath, err := reviewIntentAuthorityPayloadForOrigin153(root, origin)
	if err != nil {
		return err
	}
	key, err := loadOrCreateReviewIntentAuthorityKey153(root)
	if err != nil {
		return err
	}
	record := reviewIntentAuthorityRecord153{reviewIntentAuthorityPayload153: payload}
	record.MAC, err = reviewIntentAuthorityMAC153(key, payload)
	if err != nil {
		return err
	}
	data, err := canonicalReviewIntentAuthorityRecord153(record)
	if err != nil {
		return err
	}

	if existing, readErr := readReviewIntentAuthorityRecordBytes153(recordPath); readErr == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority conflict")
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}

	if err := os.MkdirAll(filepath.Dir(recordPath), 0o700); err != nil {
		return fmt.Errorf("REVIEW_INTENT_AUTHORITY_DIR_FAILED: %w", err)
	}
	_ = os.Chmod(filepath.Dir(recordPath), 0o700)
	f, err := os.OpenFile(recordPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := readReviewIntentAuthorityRecordBytes153(recordPath)
		if readErr != nil {
			return readErr
		}
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority conflict")
	}
	if err != nil {
		return fmt.Errorf("REVIEW_INTENT_AUTHORITY_CREATE_FAILED: %w", err)
	}
	clean := true
	defer func() {
		_ = f.Close()
		if clean {
			_ = os.Remove(recordPath)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("REVIEW_INTENT_AUTHORITY_WRITE_FAILED: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("REVIEW_INTENT_AUTHORITY_WRITE_FAILED: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("REVIEW_INTENT_AUTHORITY_WRITE_FAILED: %w", err)
	}
	clean = false
	return nil
}

func authoritativeReviewIntent153(root string, cert analysisruntime.Certificate, proposed analysisruntime.Intent) (analysisruntime.Intent, error) {
	root = filepath.Clean(root)
	recordPath, _, err := reviewIntentAuthorityRecordPath153(root, cert.RunID)
	if err != nil {
		return analysisruntime.Intent{}, err
	}
	data, err := readReviewIntentAuthorityRecordBytes153(recordPath)
	if errors.Is(err, os.ErrNotExist) {
		mirror := filepath.Join(root, ".code-harness", "runs", cert.RunID, "analysis", "review-options-origin.json")
		if _, statErr := os.Stat(mirror); statErr == nil {
			return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority missing")
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return analysisruntime.Intent{}, fmt.Errorf("REVIEW_INTENT_AUTHORITY_READ_FAILED: %w", statErr)
		}
		return proposed, nil
	}
	if err != nil {
		return analysisruntime.Intent{}, err
	}
	var record reviewIntentAuthorityRecord153
	if err := decodeStrictReviewArtifact153(data, &record); err != nil {
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: invalid original review intent authority: %w", err)
	}
	canonical, err := canonicalReviewIntentAuthorityRecord153(record)
	if err != nil {
		return analysisruntime.Intent{}, err
	}
	if !bytes.Equal(data, canonical) {
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority bytes changed")
	}
	if record.Version != reviewIntentAuthorityVersion153 || record.RunID != cert.RunID || record.ChangeSetSHA256 != cert.ChangeSetSHA256 || record.AnalysisSHA256 != cert.AnalysisSHA256 {
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority identity changed")
	}
	_, repoDigest, err := reviewIntentAuthorityRecordPath153(root, cert.RunID)
	if err != nil {
		return analysisruntime.Intent{}, err
	}
	if record.RepositorySHA256 != repoDigest {
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority repository changed")
	}
	intent, err := normalizeReviewIntent153(record.Intent)
	if err != nil || intent != record.Intent {
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority intent changed")
	}
	key, err := loadExistingReviewIntentAuthorityKey153(root)
	if err != nil {
		return analysisruntime.Intent{}, err
	}
	wantMAC, err := reviewIntentAuthorityMAC153(key, record.reviewIntentAuthorityPayload153)
	if err != nil {
		return analysisruntime.Intent{}, err
	}
	gotMAC, err := hex.DecodeString(strings.TrimSpace(record.MAC))
	if err != nil || len(gotMAC) != sha256.Size {
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority MAC invalid")
	}
	wantBytes, _ := hex.DecodeString(wantMAC)
	if !hmac.Equal(gotMAC, wantBytes) {
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority MAC mismatch")
	}
	return intent, nil
}

func reviewIntentAuthorityPayloadForOrigin153(root string, origin OptionsOrigin) (reviewIntentAuthorityPayload153, string, error) {
	intent, err := normalizeReviewIntent153(origin.Intent)
	if err != nil || intent != origin.Intent {
		return reviewIntentAuthorityPayload153{}, "", fmt.Errorf("REVIEW_OPTIONS_IDENTITY_INVALID")
	}
	recordPath, repoDigest, err := reviewIntentAuthorityRecordPath153(root, origin.RunID)
	if err != nil {
		return reviewIntentAuthorityPayload153{}, "", err
	}
	return reviewIntentAuthorityPayload153{
		Version: reviewIntentAuthorityVersion153,
		RepositorySHA256: repoDigest,
		RunID: origin.RunID,
		ChangeSetSHA256: origin.ChangeSetSHA256,
		AnalysisSHA256: origin.AnalysisSHA256,
		Intent: intent,
	}, recordPath, nil
}

func reviewIntentAuthorityRecordPath153(root, runID string) (string, string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_ROOT_FAILED: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)
	repoSum := sha256.Sum256([]byte(filepath.ToSlash(rootAbs)))
	repoDigest := hex.EncodeToString(repoSum[:])
	runSum := sha256.Sum256([]byte(strings.TrimSpace(runID)))
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	base := filepath.Join(configDir, "codea-harness", "runtime-authority", "review-intent-v1")
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	if rel, relErr := filepath.Rel(rootAbs, baseAbs); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_HOME_INVALID: authority store must be outside workspace")
	}
	return filepath.Join(baseAbs, repoDigest, hex.EncodeToString(runSum[:])+".json"), repoDigest, nil
}

func reviewIntentAuthorityKeyPath153(root string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_ROOT_FAILED: %w", err)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	keyPath, err := filepath.Abs(filepath.Join(configDir, "codea-harness", "runtime-authority", "review-intent-v1.key"))
	if err != nil {
		return "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_HOME_UNAVAILABLE: %w", err)
	}
	if rel, relErr := filepath.Rel(rootAbs, keyPath); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_HOME_INVALID: authority key must be outside workspace")
	}
	return keyPath, nil
}

func loadOrCreateReviewIntentAuthorityKey153(root string) ([]byte, error) {
	if key, err := loadExistingReviewIntentAuthorityKey153(root); err == nil {
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	path, err := reviewIntentAuthorityKeyPath153(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_DIR_FAILED: %w", err)
	}
	_ = os.Chmod(filepath.Dir(path), 0o700)
	key := make([]byte, reviewIntentAuthorityKeySize153)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_RANDOM_FAILED: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadExistingReviewIntentAuthorityKey153(root)
	}
	if err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_KEY_CREATE_FAILED: %w", err)
	}
	clean := true
	defer func() {
		_ = f.Close()
		if clean {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(key); err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_KEY_WRITE_FAILED: %w", err)
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_KEY_WRITE_FAILED: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_KEY_WRITE_FAILED: %w", err)
	}
	clean = false
	return key, nil
}

func loadExistingReviewIntentAuthorityKey153(root string) ([]byte, error) {
	path, err := reviewIntentAuthorityKeyPath153(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: review intent authority key type invalid")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: review intent authority key permissions are too broad")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_KEY_READ_FAILED: %w", err)
	}
	if len(key) != reviewIntentAuthorityKeySize153 {
		return nil, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: review intent authority key length invalid")
	}
	return key, nil
}

func readReviewIntentAuthorityRecordBytes153(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority type invalid")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("REVIEW_OPTIONS_TAMPERED: original review intent authority permissions are too broad")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_READ_FAILED: %w", err)
	}
	return data, nil
}

func reviewIntentAuthorityMAC153(key []byte, payload reviewIntentAuthorityPayload153) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("REVIEW_INTENT_AUTHORITY_ENCODE_FAILED: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func canonicalReviewIntentAuthorityRecord153(record reviewIntentAuthorityRecord153) ([]byte, error) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("REVIEW_INTENT_AUTHORITY_ENCODE_FAILED: %w", err)
	}
	return append(data, '\n'), nil
}
