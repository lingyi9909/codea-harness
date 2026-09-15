package knowledge

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
)

func Verify170(in LoadInput170, expected Manifest170) error {
	if expected.RunID != in.RunID || expected.ProjectID != in.Binding.ProjectID {
		return fmt.Errorf("KNOWLEDGE_BINDING_CHANGED: manifest identity no longer matches Runtime input")
	}
	if expected.BindingSHA256 != in.BindingSHA256 {
		return fmt.Errorf("KNOWLEDGE_BINDING_CHANGED: binding digest changed")
	}
	fresh, err := Load170(in)
	if err != nil {
		return fmt.Errorf("KNOWLEDGE_SOURCE_CHANGED: reload failed: %w", err)
	}
	if !reflect.DeepEqual(expected.Sources, fresh.Manifest.Sources) || !reflect.DeepEqual(expected.Checks, fresh.Manifest.Checks) || expected.Status != fresh.Manifest.Status || !reflect.DeepEqual(expected.Issues, fresh.Manifest.Issues) {
		return fmt.Errorf("KNOWLEDGE_SOURCE_CHANGED: depended knowledge metadata or digest changed")
	}
	return nil
}

func CanonicalBytes170(manifest Manifest170) ([]byte, error) {
	copyManifest := manifest
	sortManifest170(&copyManifest)
	return json.Marshal(copyManifest)
}

func Digest170(manifest Manifest170) (string, error) {
	encoded, err := CanonicalBytes170(manifest)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum[:]), nil
}
