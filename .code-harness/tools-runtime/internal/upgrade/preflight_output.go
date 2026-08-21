package upgrade

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultUpgradeSourceDir = ".code-harness-upgrade"

func (r Result) MarshalJSON() ([]byte, error) {
	type resultJSON Result
	out := resultJSON(r)
	out.Errors = normalizePackagePreflightErrors(defaultUpgradeSourceDir, r.Errors)
	return json.Marshal(out)
}

func normalizePackagePreflightErrors(sourceDir string, original []string) []string {
	if !hasIncompletePackageError(original) {
		return original
	}
	missing := collectMissingRequiredSource(sourceDir)
	if len(missing) == 0 {
		return original
	}

	errs := []string{"升级包不完整："}
	binaryMissing := false
	for _, rel := range missing {
		errs = append(errs, "missing: "+rel)
		if rel == "bin/codea-harness-tools.exe" || rel == "bin/ast-grep.exe" {
			binaryMissing = true
		}
	}
	if binaryMissing {
		version := "<version>"
		if b, err := os.ReadFile(filepath.Join(sourceDir, "VERSION")); err == nil && strings.TrimSpace(string(b)) != "" {
			version = strings.TrimSpace(string(b))
		}
		errs = append(errs,
			"检测到的目录可能来自 GitHub Source Code，而不是正式 Windows Release。",
			fmt.Sprintf("请使用：codea-harness-%s-windows-x64-upgrade.zip", version),
			"解压后项目根目录必须存在：",
			".code-harness-upgrade/VERSION",
			".code-harness-upgrade/bin/codea-harness-tools.exe",
			".code-harness-upgrade/bin/ast-grep.exe",
		)
	}
	return errs
}

func hasIncompletePackageError(errors []string) bool {
	for _, msg := range errors {
		if strings.Contains(msg, "incomplete upgrade package:") {
			return true
		}
	}
	return false
}

func collectMissingRequiredSource(sourceDir string) []string {
	missing := make([]string, 0)
	for _, rel := range requiredSource {
		if _, err := os.Stat(filepath.Join(sourceDir, filepath.FromSlash(rel))); err != nil {
			missing = append(missing, rel)
		}
	}
	return missing
}
