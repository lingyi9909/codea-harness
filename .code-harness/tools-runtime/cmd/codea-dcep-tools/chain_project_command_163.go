package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/nav"
)

func runProjectChainDiscover163(req chainDiscoverRequest) error {
	navigator := nav.Navigator{
		RepoRoot:    ".",
		AstGrepPath: filepath.Join(".code-harness", "bin", "ast-grep.exe"),
		Runner:      nav.ProjectExecRunner{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, source, err := chain.DiscoverProject(ctx, ".", chain.ProjectDiscoverInput{
		RunID:     req.RunID,
		Target:    req.Target,
		Navigator: navigator,
	})
	if err != nil {
		return err
	}
	for _, candidate := range result.Chains {
		candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", req.RunID, "analysis", "discovered-chains", candidate.ID+".yaml"))
		if _, err := chain.CertifyProjectCandidate(".", candidate, candidatePath, source); err != nil {
			return fmt.Errorf("certify PROJECT discovered chain candidate: %w", err)
		}
		if _, _, err := chain.LoadProjectRuntimeCandidate(".", candidatePath); err != nil {
			return fmt.Errorf("verify PROJECT discovered chain candidate: %w", err)
		}
	}
	return writeJSONAndStatus(result, result.Status == chain.DiscoveryComplete)
}
