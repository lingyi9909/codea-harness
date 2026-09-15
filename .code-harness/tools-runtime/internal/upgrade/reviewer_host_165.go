package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The patch release updates the existing Host registration in the same
// transaction as the framework. Installed hashes authorize replacement only
// when the live bytes still match; user edits never become managed implicitly.
func prepareReviewerHostPatch165(o Options) (*reviewerHostTransaction, error) {
	wanted, err := reviewerHostHashes165(o.SourceDir, "1.6.5", true)
	if err != nil {
		return nil, err
	}
	owned, err := reviewerHostHashes165(o.TargetDir, "1.6.4", false)
	if err != nil {
		return nil, err
	}
	txn := &reviewerHostTransaction{projectRoot: filepath.Dir(filepath.Clean(o.TargetDir)), sourceRoot: filepath.Join(o.SourceDir, reviewerHostUpgradeRoot)}
	for _, rel := range reviewerHostFiles164 {
		src := filepath.Join(txn.sourceRoot, filepath.FromSlash(rel))
		if err := regularHostPath165(o.SourceDir, "host/"+rel); err != nil {
			return nil, err
		}
		actual, err := hashFile(src)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(actual, wanted[rel]) {
			return nil, fmt.Errorf("Reviewer Host source hash mismatch: %s", rel)
		}
		dst := filepath.Join(txn.projectRoot, filepath.FromSlash(rel))
		if err := regularHostPath165(txn.projectRoot, rel); err != nil {
			if os.IsNotExist(err) {
				txn.snapshots = append(txn.snapshots, reviewerHostSnapshot{rel: rel})
				continue
			}
			return nil, err
		}
		info, err := os.Lstat(dst)
		if err != nil {
			return nil, err
		}
		current, err := os.ReadFile(dst)
		if err != nil {
			return nil, err
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(current))
		if !strings.EqualFold(hash, wanted[rel]) && !strings.EqualFold(hash, owned[rel]) {
			return nil, fmt.Errorf("Reviewer Host target conflict %s: current bytes do not match installed ownership; MANUAL_ACTION_REQUIRED", rel)
		}
		txn.snapshots = append(txn.snapshots, reviewerHostSnapshot{rel: rel, exists: true, data: current, mode: info.Mode()})
	}
	return txn, nil
}

func reviewerHostHashes165(root, version string, required bool) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(root, manifestPath))
	if os.IsNotExist(err) && !required {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Version    string `json:"version"`
		HostAgents struct {
			Reviewer map[string]string `json:"reviewer"`
		} `json:"hostAgents"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		return nil, err
	}
	if manifest.Version != version {
		return nil, fmt.Errorf("Reviewer Host manifest version mismatch: expected %s", version)
	}
	h := manifest.HostAgents.Reviewer
	if h == nil && !required {
		return nil, nil
	}
	if h["host"] != "opencode" || h["mode"] != "subagent" {
		return nil, fmt.Errorf("Reviewer Host registration missing or invalid")
	}
	out := map[string]string{}
	fields := [][3]string{{"path", "upgradeSource", "sha256"}, {"command", "commandUpgradeSource", "commandSha256"}, {"submissionTool", "submissionToolUpgradeSource", "submissionToolSha256"}}
	for i, rel := range reviewerHostFiles164 {
		f := fields[i]
		if h[f[0]] != rel || h[f[1]] != "host/"+rel {
			return nil, fmt.Errorf("Reviewer Host manifest path mismatch: %s", rel)
		}
		digest, err := hex.DecodeString(h[f[2]])
		if err != nil || len(digest) != sha256.Size {
			return nil, fmt.Errorf("Reviewer Host manifest hash invalid: %s", rel)
		}
		out[rel] = h[f[2]]
	}
	return out, nil
}

// Inspect each fixed path component so a symlink cannot redirect the Host
// transaction into user files outside its declared project/payload root.
func regularHostPath165(root, rel string) error {
	parts := strings.Split(rel, "/")
	p := root
	for i, part := range parts {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) || (i == len(parts)-1 && !info.Mode().IsRegular()) {
			return fmt.Errorf("Reviewer Host path is not a regular in-root resource: %s", p)
		}
	}
	return nil
}
