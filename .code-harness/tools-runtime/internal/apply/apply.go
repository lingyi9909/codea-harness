package apply

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codea-harness-tools/internal/schema"
)

const StatusApplied = "APPLIED"

type FileRequest struct {
	Path       string `json:"path"`
	BaseSha256 string `json:"baseSha256"`
}

type Request struct {
	RunID       string        `json:"runId"`
	PlanType    string        `json:"planType"`
	PlanID      string        `json:"planId"`
	DiffSha256  string        `json:"diffSha256"`
	Files       []FileRequest `json:"files"`
	UnifiedDiff string        `json:"unifiedDiff"`
}

type FileResult struct {
	Path         string `json:"path"`
	BeforeSha256 string `json:"beforeSha256"`
	AfterSha256  string `json:"afterSha256"`
}

type Result struct {
	RunID             string       `json:"runId"`
	PlanType          string       `json:"planType"`
	PlanID            string       `json:"planId"`
	DiffSha256        string       `json:"diffSha256"`
	Status            string       `json:"status,omitempty"`
	AppliedAt         string       `json:"appliedAt,omitempty"`
	Files             []FileResult `json:"files,omitempty"`
	RollbackPerformed bool         `json:"rollbackPerformed"`
}

type fileState struct {
	path       string
	abs        string
	stage      string
	before     []byte
	after      []byte
	existed    bool
	deleteFile bool
	mode       os.FileMode
}

type fileOps struct {
	replace func(src, dst string) error
}

func defaultFileOps() fileOps {
	return fileOps{replace: func(src, dst string) error {
		if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.Rename(src, dst)
	}}
}

func Apply(repoRoot string, req Request) (Result, error) {
	return applyWithOps(repoRoot, req, defaultFileOps())
}

func DecodeRequest(repoRoot string, data []byte) (Request, error) {
	schemaBytes, err := os.ReadFile(filepath.Join(repoRoot, ".code-harness", "contracts", "apply-request.schema.json"))
	if err != nil {
		return Request{}, fmt.Errorf("read apply request schema: %w", err)
	}
	if err := schema.ValidateJSON(schemaBytes, data); err != nil {
		return Request{}, err
	}
	var req Request
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return Request{}, fmt.Errorf("decode apply request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Request{}, errors.New("decode apply request: multiple JSON values are not allowed")
		}
		return Request{}, fmt.Errorf("decode apply request: %w", err)
	}
	return req, nil
}

func ApplyRequestFile(repoRoot string, inputPath string) (Result, string, error) {
	pathRunID, absInput, err := validateRequestInputPath(repoRoot, inputPath)
	if err != nil {
		return Result{}, "", err
	}
	data, err := os.ReadFile(absInput)
	if err != nil {
		return Result{}, "", fmt.Errorf("read apply request: %w", err)
	}
	req, err := DecodeRequest(repoRoot, data)
	if err != nil {
		return Result{}, "", err
	}
	if !validID(req.RunID) || !validID(req.PlanID) {
		return Result{}, "", errors.New("apply request contains invalid runId/planId")
	}
	if req.RunID != pathRunID {
		return Result{}, "", runIDPathMismatch(req.RunID, pathRunID)
	}
	if err := verifySealedPlan(repoRoot, req); err != nil {
		return Result{}, "", err
	}
	result, err := Apply(repoRoot, req)
	if err != nil {
		return result, "", err
	}
	return result, evidencePath(repoRoot, req.RunID, req.PlanID), nil
}

func applyWithOps(repoRoot string, req Request, ops fileOps) (Result, error) {
	result := Result{RunID: req.RunID, PlanType: req.PlanType, PlanID: req.PlanID, DiffSha256: req.DiffSha256, RollbackPerformed: false}
	if !validID(req.RunID) || !validID(req.PlanID) {
		return result, errors.New("INVALID_APPLY_ID")
	}
	if req.PlanType != "FIX" && req.PlanType != "TEST" {
		return result, fmt.Errorf("INVALID_PLAN_TYPE: %q", req.PlanType)
	}
	if strings.TrimSpace(req.UnifiedDiff) == "" {
		return result, errors.New("EMPTY_DIFF")
	}
	if !strings.EqualFold(req.DiffSha256, hashBytes([]byte(req.UnifiedDiff))) {
		return result, errors.New("DIFF_HASH_MISMATCH")
	}
	if len(req.Files) == 0 {
		return result, errors.New("EMPTY_FILE_SET")
	}
	if _, err := os.Stat(evidencePath(repoRoot, req.RunID, req.PlanID)); err == nil {
		return result, fmt.Errorf("PLAN_ALREADY_APPLIED: %s", req.PlanID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("check apply evidence: %w", err)
	}

	policy, err := LoadPolicy(repoRoot)
	if err != nil {
		return result, err
	}
	patches, err := parseUnifiedDiff(req.UnifiedDiff)
	if err != nil {
		return result, err
	}
	patchByPath := make(map[string]filePatch, len(patches))
	patchKeys := make(map[string]struct{}, len(patches))
	for _, patch := range patches {
		key := windowsPathKey(patch.Path)
		if _, exists := patchKeys[key]; exists {
			return result, fmt.Errorf("DUPLICATE_PATCH_PATH: %s", patch.Path)
		}
		patchKeys[key] = struct{}{}
		patchByPath[patch.Path] = patch
	}
	declared := make(map[string]FileRequest, len(req.Files))
	declaredKeys := make(map[string]struct{}, len(req.Files))
	for _, file := range req.Files {
		clean, err := safeRepoPath(file.Path)
		if err != nil {
			return result, err
		}
		key := windowsPathKey(clean)
		if _, exists := declaredKeys[key]; exists {
			return result, fmt.Errorf("DUPLICATE_DECLARED_PATH: %s", clean)
		}
		declaredKeys[key] = struct{}{}
		if err := policy.Allow(req.PlanType, clean); err != nil {
			return result, err
		}
		file.Path = clean
		declared[clean] = file
	}
	if len(declared) != len(patchByPath) {
		return result, errors.New("DECLARED_FILES_MISMATCH")
	}
	for touched := range patchByPath {
		if _, ok := declared[touched]; !ok {
			return result, fmt.Errorf("DECLARED_FILES_MISMATCH: patch touches undeclared %q", touched)
		}
	}
	for declaredPath := range declared {
		if _, ok := patchByPath[declaredPath]; !ok {
			return result, fmt.Errorf("DECLARED_FILES_MISMATCH: declared file %q not touched", declaredPath)
		}
	}

	runDir := filepath.Join(repoRoot, ".code-harness", "runs", req.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return result, fmt.Errorf("create apply run directory: %w", err)
	}
	stageDir, err := os.MkdirTemp(runDir, ".apply-"+req.PlanID+"-")
	if err != nil {
		return result, fmt.Errorf("create apply stage: %w", err)
	}
	defer os.RemoveAll(stageDir)

	paths := make([]string, 0, len(declared))
	for p := range declared {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	states := make([]fileState, 0, len(paths))
	for i, rel := range paths {
		if err := ensureNoSymlinkEscape(repoRoot, rel); err != nil {
			return result, err
		}
		fileReq := declared[rel]
		patch := patchByPath[rel]
		abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
		info, statErr := os.Stat(abs)
		existed := statErr == nil
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return result, fmt.Errorf("stat %s: %w", rel, statErr)
		}
		if patch.Create && existed {
			return result, fmt.Errorf("CREATE_TARGET_EXISTS: %s", rel)
		}
		if patch.Delete && !existed {
			return result, fmt.Errorf("DELETE_TARGET_MISSING: %s", rel)
		}
		before := []byte{}
		mode := os.FileMode(0o644)
		if existed {
			before, err = os.ReadFile(abs)
			if err != nil {
				return result, fmt.Errorf("read %s: %w", rel, err)
			}
			mode = info.Mode().Perm()
		}
		actualBase := hashBytes(before)
		if !strings.EqualFold(fileReq.BaseSha256, actualBase) {
			return result, fmt.Errorf("BASE_CHANGED: %s expected=%s actual=%s", rel, fileReq.BaseSha256, actualBase)
		}
		after, err := applyFilePatch(before, patch)
		if err != nil {
			return result, err
		}
		stagePath := filepath.Join(stageDir, fmt.Sprintf("%04d.stage", i))
		if !patch.Delete {
			if err := os.WriteFile(stagePath, after, mode); err != nil {
				return result, fmt.Errorf("stage %s: %w", rel, err)
			}
			staged, err := os.ReadFile(stagePath)
			if err != nil {
				return result, err
			}
			if hashBytes(staged) != hashBytes(after) {
				return result, fmt.Errorf("STAGE_HASH_MISMATCH: %s", rel)
			}
		}
		states = append(states, fileState{path: rel, abs: abs, stage: stagePath, before: before, after: after, existed: existed, deleteFile: patch.Delete, mode: mode})
		result.Files = append(result.Files, FileResult{Path: rel, BeforeSha256: actualBase, AfterSha256: hashBytes(after)})
	}

	for _, state := range states {
		if state.deleteFile {
			if err := os.Remove(state.abs); err != nil {
				return rollbackFailure(result, states, fmt.Errorf("apply delete %s: %w", state.path, err))
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(state.abs), 0o755); err != nil {
			return rollbackFailure(result, states, err)
		}
		if err := ops.replace(state.stage, state.abs); err != nil {
			return rollbackFailure(result, states, fmt.Errorf("apply replace %s: %w", state.path, err))
		}
	}

	result.Status = StatusApplied
	result.AppliedAt = time.Now().UTC().Format(time.RFC3339Nano)
	evidenceBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return rollbackFailure(result, states, err)
	}
	resultSchema, err := os.ReadFile(filepath.Join(repoRoot, ".code-harness", "contracts", "apply-result.schema.json"))
	if err != nil {
		return rollbackFailure(result, states, err)
	}
	if err := schema.ValidateJSON(resultSchema, evidenceBytes); err != nil {
		return rollbackFailure(result, states, err)
	}
	if err := writeEvidenceAtomic(repoRoot, req, evidenceBytes); err != nil {
		return rollbackFailure(result, states, err)
	}
	return result, nil
}

func rollbackFailure(result Result, states []fileState, cause error) (Result, error) {
	result.Status = ""
	result.AppliedAt = ""
	if err := rollback(states); err != nil {
		result.RollbackPerformed = false
		return result, fmt.Errorf("ROLLBACK_FAILED: %v; original apply error: %w", err, cause)
	}
	result.RollbackPerformed = true
	return result, cause
}

func rollback(states []fileState) error {
	var failures []string
	for _, state := range states {
		if state.existed {
			if err := os.MkdirAll(filepath.Dir(state.abs), 0o755); err != nil {
				failures = append(failures, fmt.Sprintf("%s mkdir: %v", state.path, err))
				continue
			}
			if err := os.WriteFile(state.abs, state.before, state.mode); err != nil {
				failures = append(failures, fmt.Sprintf("%s restore: %v", state.path, err))
				continue
			}
			got, err := os.ReadFile(state.abs)
			if err != nil || hashBytes(got) != hashBytes(state.before) {
				failures = append(failures, fmt.Sprintf("%s restore verification failed", state.path))
			}
			continue
		}
		if err := os.Remove(state.abs); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Sprintf("%s remove created file: %v", state.path, err))
			continue
		}
		if _, err := os.Lstat(state.abs); !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Sprintf("%s created file still exists after rollback", state.path))
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func writeEvidenceAtomic(repoRoot string, req Request, data []byte) error {
	path := evidencePath(repoRoot, req.RunID, req.PlanID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create apply evidence directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write apply evidence: %w", err)
	}
	defer os.Remove(tmp)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("PLAN_ALREADY_APPLIED: %s", req.PlanID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("commit apply evidence: %w", err)
	}
	return nil
}

func evidencePath(repoRoot, runID, planID string) string {
	return filepath.Join(repoRoot, ".code-harness", "runs", runID, "evidence", "apply", planID+".json")
}

func windowsPathKey(value string) string {
	return strings.ToLower(strings.ReplaceAll(filepath.Clean(value), "\\", "/"))
}

func validID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
			continue
		}
		return false
	}
	return true
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
