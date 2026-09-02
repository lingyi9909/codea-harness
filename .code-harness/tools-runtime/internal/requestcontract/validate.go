package requestcontract

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"codea-harness-tools/internal/schema"
)

func Validate(name string, data []byte) error {
	contractPath, err := locate(name)
	if err != nil {
		return err
	}
	contractBytes, err := os.ReadFile(contractPath)
	if err != nil {
		return fmt.Errorf("REQUEST_CONTRACT_READ_FAILED: %s: %w", name, err)
	}
	if err := schema.ValidateJSON(contractBytes, data); err != nil {
		return fmt.Errorf("REQUEST_CONTRACT_INVALID: %s: %w", name, err)
	}
	return nil
}

func locate(name string) (string, error) {
	installed := filepath.Join(".code-harness", "contracts", name)
	if _, err := os.Stat(installed); err == nil {
		return installed, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("REQUEST_CONTRACT_READ_FAILED: %s: %w", name, err)
	}

	// Go tests often chdir into a temporary repository that intentionally copies
	// only the historical contracts under test. Resolve the checked-out source
	// contract as a test fallback; production builds use the installed path above.
	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		source := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "contracts", name))
		if _, err := os.Stat(source); err == nil {
			return source, nil
		}
	}
	return "", fmt.Errorf("REQUEST_CONTRACT_READ_FAILED: %s: contract not found", name)
}
