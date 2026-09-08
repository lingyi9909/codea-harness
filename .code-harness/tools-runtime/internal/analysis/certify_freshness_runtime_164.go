package analysis

import "codea-harness-tools/internal/changeset"

type certificationFreshnessRuntime164 interface {
	VerifyFreshness(root string, snapshot changeset.Snapshot) error
}

func (defaultCertificationRuntime153) VerifyFreshness(root string, snapshot changeset.Snapshot) error {
	return changeset.VerifyFreshness(root, snapshot)
}
