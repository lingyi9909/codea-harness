package workspace

import (
	"errors"
	"fmt"
	"strings"
)

var errLocalParentIdentity = errors.New("local Maven parent identity unresolved or mismatched")

func verifyDeclaredLocalParent(parent effectivePOM, declared pomParent, childProperties pomProperties) error {
	declaredProps := make(map[string]string, len(childProperties))
	for key, value := range childProperties {
		declaredProps[key] = strings.TrimSpace(value)
	}

	declaredGroup, groupOK := resolveValue(declared.GroupID, declaredProps)
	declaredArtifact, artifactOK := resolveValue(declared.ArtifactID, declaredProps)
	declaredVersion, versionOK := resolveValue(declared.Version, declaredProps)
	parentGroup, parentGroupOK := resolveValue(parent.GroupID, parent.Properties)
	parentArtifact, parentArtifactOK := resolveValue(parent.ArtifactID, parent.Properties)
	parentVersion, parentVersionOK := resolveValue(parent.Version, parent.Properties)

	if !groupOK || !artifactOK || !versionOK || !parentGroupOK || !parentArtifactOK || !parentVersionOK ||
		strings.TrimSpace(declaredGroup) == "" || strings.TrimSpace(declaredArtifact) == "" || strings.TrimSpace(declaredVersion) == "" ||
		strings.TrimSpace(parentGroup) == "" || strings.TrimSpace(parentArtifact) == "" || strings.TrimSpace(parentVersion) == "" {
		return fmt.Errorf("%w: declared or loaded parent GAV is unresolved", errLocalParentIdentity)
	}
	if declaredGroup != parentGroup || declaredArtifact != parentArtifact || declaredVersion != parentVersion {
		return fmt.Errorf("%w: declared=%s:%s:%s loaded=%s:%s:%s", errLocalParentIdentity,
			declaredGroup, declaredArtifact, declaredVersion, parentGroup, parentArtifact, parentVersion)
	}
	return nil
}
