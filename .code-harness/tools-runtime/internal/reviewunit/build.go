package reviewunit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/projectpath"
	"codea-harness-tools/internal/reviewscope"
	"codea-harness-tools/internal/schema"
	"codea-harness-tools/internal/symbolid"
)

var runID160 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type buildFacts160 struct {
	runID          string
	harnessVersion string
	analysis       analysisruntime.ChangeAnalysis
	cert           analysisruntime.Certificate
	scope          reviewscope.Selection
	scopeSHA256    string
	snapshot       changeset.Snapshot
}

type analysisMeta160 struct {
	ReviewScope struct {
		IncludeWorkingTree bool `json:"includeWorkingTree"`
	} `json:"reviewScope"`
}

type symbolFact160 struct {
	workspace string
	path      string
	role      string
}

func Build(input BuildInput) (Manifest, error) {
	facts, err := loadFacts160(input)
	if err != nil { return Manifest{}, err }
	return buildFromFacts160(facts)
}

func Load(input BuildInput) (Manifest, error) {
	root, runID, _, err := normalizeBuildInput160(input)
	if err != nil { return Manifest{}, err }
	artifact := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "review-units.json")
	raw, err := os.ReadFile(artifact)
	if err != nil { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: read manifest: %w", err) }
	schemaBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "contracts", "review-unit.schema.json"))
	if err != nil { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: read schema: %w", err) }
	if err := schema.ValidateJSON(schemaBytes, raw); err != nil { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: schema invalid: %w", err) }
	facts, err := loadFacts160(input)
	if err != nil { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: upstream authority changed: %w", err) }
	return loadCanonicalWithFacts160(raw, facts)
}

func loadFacts160(input BuildInput) (buildFacts160, error) {
	root, runID, certifiedRunID, err := normalizeBuildInput160(input)
	if err != nil { return buildFacts160{}, err }
	analysisRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", certifiedRunID, "analysis", "change-analysis.json"))
	analysisValue, cert, err := analysisruntime.LoadCertified(root, analysisRel)
	if err != nil { return buildFacts160{}, err }
	if cert.RunID != runID { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: certified analysis runId=%s review runId=%s", cert.RunID, runID) }
	analysisPath := filepath.Join(root, filepath.FromSlash(analysisRel))
	analysisBytes, err := os.ReadFile(analysisPath)
	if err != nil { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_ANALYSIS_READ_FAILED: %w", err) }
	var meta analysisMeta160
	if err := json.Unmarshal(analysisBytes, &meta); err != nil { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_ANALYSIS_DECODE_FAILED: %w", err) }

	scopePath := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "review-scope.json")
	var scope reviewscope.Selection
	var scopeSHA string
	scopeBytes, scopeErr := os.ReadFile(scopePath)
	switch {
	case scopeErr == nil:
		scope, err = reviewscope.Verify(scopeBytes, analysisBytes)
		if err != nil { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: %w", err) }
		canonicalScope, err := canonicalJSON160(scopeBytes)
		if err != nil { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: canonicalize review scope: %w", err) }
		scopeSHA = hash160(canonicalScope)
	case os.IsNotExist(scopeErr):
		scope, err = reviewscope.BuildFullSelection(analysisBytes)
		if err != nil { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: build FULL scope: %w", err) }
	default:
		return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: read review scope: %w", scopeErr)
	}

	snapshot, err := changeset.Compute(root, cert.BaseRef, meta.ReviewScope.IncludeWorkingTree)
	if err != nil { return buildFacts160{}, err }
	if snapshot.SHA256 != cert.ChangeSetSHA256 || snapshot.Head != cert.Head { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_STALE: Change Set no longer matches Certified ChangeAnalysis") }
	versionBytes, err := os.ReadFile(filepath.Join(root, ".code-harness", "VERSION"))
	if err != nil { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_VERSION_UNAVAILABLE: %w", err) }
	version := strings.TrimSpace(string(versionBytes))
	if version == "" || version != cert.RuntimeVersion { return buildFacts160{}, fmt.Errorf("REVIEW_UNIT_STALE: runtime version=%q certificate=%q", version, cert.RuntimeVersion) }
	return buildFacts160{runID: runID, harnessVersion: version, analysis: analysisValue, cert: cert, scope: scope, scopeSHA256: scopeSHA, snapshot: snapshot}, nil
}

func normalizeBuildInput160(input BuildInput) (string, string, string, error) {
	root := strings.TrimSpace(input.RepoRoot)
	if root == "" { root = "." }
	root = filepath.Clean(root)
	runID := strings.TrimSpace(input.RunID)
	if !runID160.MatchString(runID) { return "", "", "", fmt.Errorf("REVIEW_UNIT_RUN_ID_INVALID: %q", input.RunID) }
	certifiedRunID := strings.TrimSpace(input.CertifiedRunID)
	if certifiedRunID == "" { certifiedRunID = runID }
	if !runID160.MatchString(certifiedRunID) || certifiedRunID != runID { return "", "", "", fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: ReviewUnit and Certified ChangeAnalysis must use the same runId") }
	return root, runID, certifiedRunID, nil
}

func buildFromFacts160(facts buildFacts160) (Manifest, error) {
	mode := Mode(strings.ToUpper(strings.TrimSpace(facts.scope.Mode)))
	if mode != ModeFull && mode != ModeTargeted { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: unsupported mode %q", facts.scope.Mode) }
	if facts.cert.Intent != nil && strings.EqualFold(strings.TrimSpace(facts.cert.Intent.Mode), string(ModeTargeted)) && mode != ModeTargeted {
		return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: TARGETED Certified Analysis requires verified TARGETED review scope")
	}
	if facts.cert.RunID != facts.runID || facts.cert.ChangeSetSHA256 != facts.snapshot.SHA256 { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: certified identity does not match build facts") }

	changed := map[string]changeset.File{}
	for _, file := range facts.snapshot.Files {
		p, ok := safePath160(file.Path)
		if !ok { continue }
		file.Path = p
		changed[p] = file
	}
	roles := map[string]string{}
	currentEvidence := map[string]bool{}
	dependencyPaths := map[string]bool{}
	addCurrentRole := func(rawPath, rawRole string) error {
		p, ok := safePath160(rawPath)
		if !ok { return fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: invalid current path %q", rawPath) }
		role := strings.TrimSpace(rawRole)
		if role == "" { return fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: missing role for %s", p) }
		if previous, exists := roles[p]; exists && previous != role { return fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: conflicting roles for %s: %s/%s", p, previous, role) }
		roles[p] = role
		currentEvidence[p] = true
		return nil
	}
	for _, file := range facts.analysis.ChangedFiles { if err := addCurrentRole(file.Path, file.Role); err != nil { return Manifest{}, err } }
	for _, file := range facts.analysis.ReviewCoverage.ReviewedFiles { if err := addCurrentRole(file.Path, file.Role); err != nil { return Manifest{}, err } }

	symbols := map[string]symbolFact160{}
	symbolsByIdentity := map[string]symbolFact160{}
	ambiguousSymbols := map[string]bool{}
	for _, loc := range facts.analysis.SymbolLocations {
		ref, ok := symbolid.FromLocation(loc.Workspace, loc.Path, loc.Symbol)
		if !ok { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: invalid symbol location %q/%q", loc.Symbol, loc.Path) }
		fact := symbolFact160{workspace: ref.Workspace, path: ref.Path, role: strings.TrimSpace(loc.Role)}
		identity, _ := symbolid.Key(ref)
		if previous, exists := symbolsByIdentity[identity]; exists && previous != fact { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: conflicting exact symbol identity %s", ref.Symbol) }
		symbolsByIdentity[identity] = fact
		if !ambiguousSymbols[ref.Symbol] {
			if previous, exists := symbols[ref.Symbol]; exists && previous != fact {
				delete(symbols, ref.Symbol)
				ambiguousSymbols[ref.Symbol] = true
			} else if !exists {
				symbols[ref.Symbol] = fact
			}
		}
		if ref.Workspace == symbolid.CurrentWorkspace {
			if err := addCurrentRole(ref.Path, fact.role); err != nil { return Manifest{}, err }
		} else { dependencyPaths[ref.Path] = true }
	}
	resolveSymbol := func(symbol string, exact *analysisruntime.SymbolRef) (symbolFact160, error) {
		symbol = strings.TrimSpace(symbol)
		if exact != nil {
			ref, ok := symbolid.Normalize(*exact)
			if !ok || ref.Symbol != symbol { return symbolFact160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: invalid exact ref for branch symbol %s", symbol) }
			identity, _ := symbolid.Key(ref)
			fact, exists := symbolsByIdentity[identity]
			if !exists { return symbolFact160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: exact branch symbol %s at %s/%s lacks verified location", symbol, ref.Workspace, ref.Path) }
			return fact, nil
		}
		if ambiguousSymbols[symbol] { return symbolFact160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: ambiguous symbol %s requires path-qualified ref", symbol) }
		fact, exists := symbols[symbol]
		if !exists { return symbolFact160{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: branch symbol %s lacks verified location", symbol) }
		return fact, nil
	}
	for _, relation := range facts.analysis.ResourceRelations { if err := addCurrentRole(relation.Path, relation.Role); err != nil { return Manifest{}, err } }

	allowed := map[string]bool{}
	switch mode {
	case ModeFull:
		for _, file := range facts.analysis.ReviewCoverage.ReviewedFiles { if p, ok := safePath160(file.Path); ok && isFindingScopePath160(p) { allowed[p] = true } }
		for _, file := range facts.analysis.ChangedFiles { if p, ok := safePath160(file.Path); ok && isFindingScopePath160(p) { allowed[p] = true } }
	case ModeTargeted:
		if len(facts.scope.SelectedCallChains) == 0 || len(facts.scope.ScopedFiles) == 0 { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: TARGETED scope requires selectedCallChains and scopedFiles") }
		for _, raw := range facts.scope.ScopedFiles {
			p, ok := safePath160(raw)
			if !ok || !isFindingScopePath160(p) { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: TARGETED path %q is outside Finding scope", raw) }
			if dependencyPaths[p] && !currentEvidence[p] { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: dependency workspace path %s cannot enter files[]", p) }
			if !currentEvidence[p] { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: scoped file %s lacks current-workspace evidence", p) }
			allowed[p] = true
		}
	}

	chains := selectedChains160(facts.analysis.CallChains, facts.scope, mode)
	units := make([]Unit, 0, len(chains)+len(allowed))
	covered := map[string]bool{}
	seenCore := map[string]bool{}
	for _, rawChain := range chains {
		canonicalChain := normalizeChain160(rawChain)
		if canonicalChain.EntryPoint == "" || len(canonicalChain.Chain) == 0 { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: empty confirmed branch") }
		if len(canonicalChain.ChainRefs) != 0 && len(canonicalChain.ChainRefs) != len(canonicalChain.Chain) { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: branch %s has invalid chainRefs length", canonicalChain.EntryPoint) }
		core := authorityChainKey160(canonicalChain)
		if seenCore[core] { continue }
		seenCore[core] = true
		unit := Unit{EntryPoint: canonicalChain.EntryPoint, Chain: canonicalChain.Chain, Files: []FileRef{}}
		branchSymbols := map[string]bool{}
		for i, symbol := range unit.Chain {
			branchSymbols[symbol] = true
			var exact *analysisruntime.SymbolRef
			if len(canonicalChain.ChainRefs) > 0 { ref := canonicalChain.ChainRefs[i]; exact = &ref }
			loc, err := resolveSymbol(symbol, exact)
			if err != nil { return Manifest{}, err }
			if loc.workspace != symbolid.CurrentWorkspace {
				unit.ContextSymbols = append(unit.ContextSymbols, symbol)
				continue
			}
			if !allowed[loc.path] { continue }
			file, err := fileRef160(loc.path, roles, changed)
			if err != nil { return Manifest{}, err }
			unit.Files = append(unit.Files, file)
			covered[loc.path] = true
		}
		for _, relation := range facts.analysis.ResourceRelations {
			from := strings.TrimSpace(relation.FromSymbol)
			if !branchSymbols[from] { continue }
			if ambiguousSymbols[from] { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: resource relation fromSymbol %s is ambiguous across module paths", from) }
			p, ok := safePath160(relation.Path)
			if !ok || !allowed[p] { continue }
			file, err := fileRef160(p, roles, changed)
			if err != nil { return Manifest{}, err }
			unit.Files = append(unit.Files, file)
			covered[p] = true
		}
		unit = normalizeUnit160(unit)
		if mode == ModeTargeted && len(unit.Files) == 0 { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: selected branch %s has no scoped current-project files", unit.EntryPoint) }
		unit.ChangedHunks = hunksForFiles160(unit.Files, changed)
		digest, err := canonicalUnitDigest160(unit)
		if err != nil { return Manifest{}, err }
		unit.ID = "RU-" + digest
		units = append(units, normalizeUnit160(unit))
	}

	paths := make([]string, 0, len(allowed))
	for p := range allowed { paths = append(paths, p) }
	sort.Strings(paths)
	for _, p := range paths {
		if covered[p] { continue }
		if mode == ModeTargeted { return Manifest{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: scoped file %s is not bound to a selected verified branch", p) }
		file, err := fileRef160(p, roles, changed)
		if err != nil { return Manifest{}, err }
		unit := Unit{Files: []FileRef{file}}
		unit.ChangedHunks = hunksForFiles160(unit.Files, changed)
		digest, err := canonicalUnitDigest160(unit)
		if err != nil { return Manifest{}, err }
		unit.ID = "RU-FILE-" + digest
		units = append(units, normalizeUnit160(unit))
	}

	manifest := Manifest{RunID: facts.runID, HarnessVersion: facts.harnessVersion, Mode: mode, ChangeSetSHA256: facts.cert.ChangeSetSHA256, ChangeAnalysisSHA256: facts.cert.AnalysisSHA256, ReviewScopeSHA256: facts.scopeSHA256, Units: units}
	return sealManifest160(manifest)
}

func selectedChains160(all []analysisruntime.CallChain, scope reviewscope.Selection, mode Mode) []analysisruntime.CallChain {
	out := []analysisruntime.CallChain{}
	if mode == ModeFull {
		out = append(out, all...)
	} else {
		for _, chain := range scope.SelectedCallChains {
			converted := analysisruntime.CallChain{EntryPoint: chain.EntryPoint, Chain: append([]string(nil), chain.Chain...), ChainRefs: append([]analysisruntime.SymbolRef(nil), chain.ChainRefs...)}
			if chain.EntryPointRef != nil { ref := analysisruntime.SymbolRef(*chain.EntryPointRef); converted.EntryPointRef = &ref }
			out = append(out, converted)
		}
	}
	sort.Slice(out, func(i, j int) bool { return authorityChainKey160(out[i]) < authorityChainKey160(out[j]) })
	return out
}

func normalizeChain160(chain analysisruntime.CallChain) analysisruntime.CallChain {
	entry := strings.TrimSpace(chain.EntryPoint)
	out := make([]string, 0, len(chain.Chain))
	refs := make([]analysisruntime.SymbolRef, 0, len(chain.ChainRefs))
	for i, raw := range chain.Chain {
		symbol := strings.TrimSpace(raw)
		if symbol == "" { continue }
		out = append(out, symbol)
		if len(chain.ChainRefs) > 0 && i < len(chain.ChainRefs) { refs = append(refs, chain.ChainRefs[i]) }
	}
	result := analysisruntime.CallChain{EntryPoint: entry, Chain: out, ChainRefs: refs}
	if chain.EntryPointRef != nil { ref := *chain.EntryPointRef; result.EntryPointRef = &ref }
	return result
}

func authorityChainKey160(chain analysisruntime.CallChain) string {
	parts := []string{strings.TrimSpace(chain.EntryPoint)}
	parts = append(parts, chain.Chain...)
	if chain.EntryPointRef != nil {
		if key, ok := symbolid.Key(*chain.EntryPointRef); ok { parts = append(parts, "entryRef="+key) }
	}
	for _, ref := range chain.ChainRefs {
		if key, ok := symbolid.Key(ref); ok { parts = append(parts, "chainRef="+key) }
	}
	return strings.Join(parts, "\x00")
}

func fileRef160(p string, roles map[string]string, changed map[string]changeset.File) (FileRef, error) {
	role := strings.TrimSpace(roles[p])
	if role == "" { return FileRef{}, fmt.Errorf("REVIEW_UNIT_SCOPE_VIOLATION: no verified role for %s", p) }
	_, isChanged := changed[p]
	return FileRef{Path: p, Role: role, Changed: isChanged, Workspace: symbolid.CurrentWorkspace}, nil
}

func hunksForFiles160(files []FileRef, changed map[string]changeset.File) []HunkRef {
	var out []HunkRef
	for _, file := range files {
		change, ok := changed[file.Path]
		if !ok { continue }
		for _, hunk := range change.Hunks { out = append(out, HunkRef{Path: file.Path, NewStart: hunk.NewStart, NewLines: hunk.NewLines}) }
	}
	return normalizeUnit160(Unit{Files: []FileRef{}, ChangedHunks: out}).ChangedHunks
}

func loadCanonicalWithFacts160(raw []byte, facts buildFacts160) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: decode manifest: %w", err) }
	canonical, err := CanonicalBytes(manifest)
	if err != nil { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: canonicalize manifest: %w", err) }
	if !bytes.Equal(raw, canonical) { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: manifest bytes are not canonical") }
	if err := verifyManifestHash160(manifest); err != nil { return Manifest{}, err }
	current, err := buildFromFacts160(facts)
	if err != nil { return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: rebuild current authority: %w", err) }
	if manifest.RunID != current.RunID || manifest.HarnessVersion != current.HarnessVersion || manifest.Mode != current.Mode || manifest.ChangeSetSHA256 != current.ChangeSetSHA256 || manifest.ChangeAnalysisSHA256 != current.ChangeAnalysisSHA256 || manifest.ReviewScopeSHA256 != current.ReviewScopeSHA256 || manifest.SHA256 != current.SHA256 {
		return Manifest{}, fmt.Errorf("REVIEW_UNIT_STALE: manifest no longer matches Certified ChangeAnalysis / Review Scope")
	}
	return normalizeManifest160(manifest), nil
}

func normalizeWorkspace160(value string) string { return symbolid.NormalizeWorkspace(value) }

func safePath160(value string) (string, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || path.IsAbs(value) { return "", false }
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") { return "", false }
	return clean, true
}

func isFindingScopePath160(p string) bool {
	p, ok := safePath160(p)
	return ok && projectpath.IsReviewPath(p)
}

func canonicalJSON160(raw []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil { return nil, err }
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil { return nil, err }
	return append(data, '\n'), nil
}
