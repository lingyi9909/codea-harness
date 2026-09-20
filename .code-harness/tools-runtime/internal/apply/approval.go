package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SealRequestFile(repoRoot, inputPath string) (string, error) {
	pathRunID, absInput, err := validateRequestInputPath(repoRoot, inputPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(absInput)
	if err != nil {
		return "", fmt.Errorf("read apply request: %w", err)
	}
	req, err := DecodeRequest(repoRoot, data)
	if err != nil {
		return "", err
	}
	if !validID(req.RunID) || !validID(req.PlanID) {
		return "", errors.New("apply request contains invalid runId/planId")
	}
	if req.RunID != pathRunID {
		return "", runIDPathMismatch(req.RunID, pathRunID)
	}
	if err := validateSealableIdentity(req); err != nil {
		return "", err
	}

	sealedPath := sealedPlanPath(repoRoot, req.RunID, req.PlanID)
	if err := os.MkdirAll(filepath.Dir(sealedPath), 0o755); err != nil {
		return "", fmt.Errorf("create sealed plan directory: %w", err)
	}
	sealedBytes, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode sealed plan: %w", err)
	}
	f, err := os.OpenFile(sealedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o444)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("PLAN_ALREADY_SEALED: %s", req.PlanID)
	}
	if err != nil {
		return "", fmt.Errorf("create sealed plan: %w", err)
	}
	committed := false
	defer func() {
		_ = f.Close()
		if !committed {
			_ = os.Remove(sealedPath)
		}
	}()
	if _, err := f.Write(sealedBytes); err != nil {
		return "", fmt.Errorf("write sealed plan: %w", err)
	}
	if err := f.Sync(); err != nil {
		return "", fmt.Errorf("sync sealed plan: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close sealed plan: %w", err)
	}
	if err := os.Chmod(sealedPath, 0o444); err != nil {
		return "", fmt.Errorf("protect sealed plan: %w", err)
	}
	committed = true
	return sealedPath, nil
}

func verifySealedPlan(repoRoot string, req Request) error {
	sealedPath := sealedPlanPath(repoRoot, req.RunID, req.PlanID)
	data, err := os.ReadFile(sealedPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SEALED_PLAN_NOT_FOUND: %s", req.PlanID)
	}
	if err != nil {
		return fmt.Errorf("read sealed plan: %w", err)
	}
	approved, err := DecodeRequest(repoRoot, data)
	if err != nil {
		return fmt.Errorf("SEALED_PLAN_INVALID: %w", err)
	}
	if approved.RunID != req.RunID {
		return errors.New("APPROVAL_IDENTITY_MISMATCH: runId")
	}
	if approved.PlanID != req.PlanID {
		return errors.New("APPROVAL_IDENTITY_MISMATCH: planId")
	}
	if approved.PlanType != req.PlanType {
		return errors.New("APPROVAL_IDENTITY_MISMATCH: planType")
	}
	if approved.UnifiedDiff != req.UnifiedDiff {
		return errors.New("APPROVAL_IDENTITY_MISMATCH: unifiedDiff exact bytes")
	}
	if approved.DiffSha256 != req.DiffSha256 {
		return errors.New("APPROVAL_IDENTITY_MISMATCH: diffSha256")
	}
	if len(approved.Files) != len(req.Files) {
		return errors.New("APPROVAL_IDENTITY_MISMATCH: files")
	}
	for i := range approved.Files {
		if approved.Files[i].Path != req.Files[i].Path {
			return fmt.Errorf("APPROVAL_IDENTITY_MISMATCH: files[%d].path", i)
		}
		if approved.Files[i].BaseSha256 != req.Files[i].BaseSha256 {
			return fmt.Errorf("APPROVAL_IDENTITY_MISMATCH: files[%d].baseSha256", i)
		}
	}
	return nil
}

func validateSealableIdentity(req Request) error {
	if req.PlanType != "FIX" && req.PlanType != "TEST" {
		return fmt.Errorf("INVALID_PLAN_TYPE: %q", req.PlanType)
	}
	if strings.TrimSpace(req.UnifiedDiff) == "" {
		return errors.New("EMPTY_DIFF")
	}
	if !strings.EqualFold(req.DiffSha256, hashBytes([]byte(req.UnifiedDiff))) {
		return errors.New("DIFF_HASH_MISMATCH")
	}
	if len(req.Files) == 0 {
		return errors.New("EMPTY_FILE_SET")
	}
	seen := make(map[string]struct{}, len(req.Files))
	for _, file := range req.Files {
		clean, err := safeRepoPath(file.Path)
		if err != nil {
			return err
		}
		key := windowsPathKey(clean)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("DUPLICATE_DECLARED_PATH: %s", clean)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateRequestInputPath(repoRoot, inputPath string) (string, string, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	var absInput string
	if filepath.IsAbs(inputPath) {
		absInput = filepath.Clean(inputPath)
	} else {
		absInput = filepath.Join(absRoot, filepath.Clean(inputPath))
	}
	rel, err := filepath.Rel(absRoot, absInput)
	if err != nil {
		return "", "", invalidRequestPathError()
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 5 || !strings.EqualFold(parts[0], ".code-harness") || !strings.EqualFold(parts[1], "runs") || !strings.EqualFold(parts[3], "requests") || !strings.EqualFold(filepath.Ext(parts[4]), ".json") || parts[4] == ".json" || !validID(parts[2]) {
		return "", "", invalidRequestPathError()
	}
	if err := ensureNoSymlinkEscape(absRoot, rel); err != nil {
		return "", "", fmt.Errorf("apply input path rejected before read: %w", err)
	}
	return parts[2], absInput, nil
}

func invalidRequestPathError() error {
	return errors.New("apply input must be under .code-harness/runs/<runId>/requests/*.json")
}

func runIDPathMismatch(bodyRunID, pathRunID string) error {
	return fmt.Errorf("RUN_ID_PATH_MISMATCH: body runId %q does not match request path runId %q; apply input must be under .code-harness/runs/<runId>/requests/*.json", bodyRunID, pathRunID)
}

func sealedPlanPath(repoRoot, runID, planID string) string {
	return filepath.Join(repoRoot, ".code-harness", "runs", runID, "sealed-plans", planID+".json")
}
