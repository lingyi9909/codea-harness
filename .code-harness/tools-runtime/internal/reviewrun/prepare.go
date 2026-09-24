package reviewrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"codea-harness-tools/internal/nav"
)

type preparedOptions180 struct {
	SchemaVersion       int      `json:"schemaVersion"`
	Intent              Intent   `json:"intent"`
	ProjectRoot         string   `json:"projectRoot"`
	SourceFingerprint   string   `json:"sourceFingerprint"`
	SourcePaths         []string `json:"sourcePaths"`
	NavigationProcesses int      `json:"navigationProcesses"`
	Options             Options  `json:"options"`
}

type rootNavRunner180 struct {
	root  string
	mu    sync.Mutex
	count int
}

func (r *rootNavRunner180) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.root
	var se strings.Builder
	cmd.Stderr = &se
	out, err := cmd.Output()
	if err != nil && se.Len() > 0 {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(se.String()))
	}
	return out, err
}

func (r *rootNavRunner180) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func Prepare(ctx context.Context, root, runID string, intent Intent) (Options, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}
	intent, err := normalizeIntent180(intent)
	if err != nil {
		return Options{}, err
	}
	runDir, state, err := loadRun(root, runID)
	if err != nil {
		return Options{}, err
	}
	if state.Cancelled {
		return Options{}, fmt.Errorf("REVIEW_PREPARE_CANCELLED")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Options{}, err
	}
	rootAbs = filepath.Clean(rootAbs)

	if old, e := loadPreparedOptions180(runDir); e == nil {
		if old.Intent != intent || !sameProjectPath180(old.ProjectRoot, rootAbs) {
			return Options{}, fmt.Errorf("REVIEW_PREPARE_INTENT_CONFLICT")
		}
		same, x := sourceSnapshotMatches180(rootAbs, old.SourcePaths, old.SourceFingerprint)
		if x != nil {
			return Options{}, x
		}
		if old.Options.DiscoveryComplete && same {
			return old.Options, nil
		}
	}

	sources, err := sourcePaths180(rootAbs)
	if err != nil {
		return Options{}, err
	}
	before, _, err := fingerprintPaths180(rootAbs, sources)
	if err != nil {
		return Options{}, err
	}
	javaFiles := filterExt180(sources, ".java")
	changedSources := []string{}
	if intent.Mode == "CHANGES" {
		changedSources, err = changedSourceFiles180(ctx, rootAbs)
		if err != nil {
			return persistPrepared180(runDir, state, intent, rootAbs, sources, before, nil, false, []string{"CHANGESET_DISCOVERY_FAILED: " + err.Error()}, 0)
		}
		if intent.Target != "" && len(changedSources) == 0 {
			return Options{}, fmt.Errorf("REVIEW_TARGET_NO_RELEVANT_CHANGES")
		}
	}

	astPath, err := astGrepPath180(rootAbs)
	if err != nil {
		return persistPrepared180(runDir, state, intent, rootAbs, sources, before, nil, false, []string{"AST_GREP_UNAVAILABLE: " + err.Error()}, 0)
	}
	if len(javaFiles) == 0 {
		return persistPrepared180(runDir, state, intent, rootAbs, sources, before, []Chain{}, false, []string{"ENTRYPOINT_NOT_FOUND"}, 0)
	}

	runner := &rootNavRunner180{root: rootAbs}
	n := nav.Navigator{RepoRoot: rootAbs, AstGrepPath: astPath, Runner: runner}
	endpoints, err := n.FindControllerEndpointsBatch(ctx, javaFiles)
	if err != nil {
		return persistPrepared180(runDir, state, intent, rootAbs, sources, before, nil, false, []string{"ENTRYPOINT_DISCOVERY_FAILED: " + err.Error()}, runner.Count())
	}
	endpoints = filterEndpoints180(endpoints, intent.Target)
	if len(endpoints) == 0 {
		return persistPrepared180(runDir, state, intent, rootAbs, sources, before, []Chain{}, false, []string{"ENTRYPOINT_NOT_FOUND"}, runner.Count())
	}

	scope := scanScope180(javaFiles)
	calls, err := n.FindDirectMethodCallsBatch180(ctx, scope)
	if err != nil {
		return persistPrepared180(runDir, state, intent, rootAbs, sources, before, nil, false, []string{"CALL_DISCOVERY_FAILED: " + err.Error()}, runner.Count())
	}
	receivers := []string{}
	for _, facts := range calls {
		for _, f := range facts {
			if f.Resolved && f.ReceiverType != "" {
				receivers = append(receivers, f.ReceiverType)
			}
		}
	}
	impls, err := n.FindImplementationTypesBatch180(ctx, uniqueStrings180(receivers), scope)
	if err != nil {
		return persistPrepared180(runDir, state, intent, rootAbs, sources, before, nil, false, []string{"IMPLEMENTATION_DISCOVERY_FAILED: " + err.Error()}, runner.Count())
	}

	targetSymbols := []string{}
	infoSymbols := append([]string{}, receivers...)
	for _, ep := range endpoints {
		for _, c := range calls[ep.Symbol] {
			if !c.Resolved {
				continue
			}
			symbol := c.TargetSymbol
			if xs := impls[c.ReceiverType]; len(xs) == 1 {
				symbol = xs[0].Symbol + "." + c.Method
			}
			for _, sc := range calls[symbol] {
				if sc.Resolved {
					targetSymbols = append(targetSymbols, sc.TargetSymbol)
					infoSymbols = append(infoSymbols, sc.ReceiverType)
				}
			}
		}
	}
	infoSymbols = append(infoSymbols, targetSymbols...)
	infos := map[string]nav.SymbolInfo{}
	if len(infoSymbols) > 0 {
		infos, err = n.GetSymbolInfos(ctx, uniqueStrings180(infoSymbols), scope)
		if err != nil && !errors.Is(err, nav.ErrSymbolNotFound) {
			return persistPrepared180(runDir, state, intent, rootAbs, sources, before, nil, false, []string{"TARGET_DISCOVERY_FAILED: " + err.Error()}, runner.Count())
		}
	}

	xmlIndex := indexMapperXML180(rootAbs, filterExt180(sources, ".xml"))
	changedSet := pathSet180(changedSources)
	chains := make([]Chain, 0, len(endpoints))
	affected := map[string]bool{}
	for _, ep := range endpoints {
		ch := Chain{
			Name:       ep.Symbol,
			Nodes:      []Node{{Path: filepath.ToSlash(ep.Path), Symbol: ep.Symbol, Role: "CONTROLLER", Workspace: "current"}},
			Unresolved: []string{},
		}
		if changedSet[filepath.ToSlash(ep.Path)] {
			affected[ch.Name] = true
		}
		facts := calls[ep.Symbol]
		if len(facts) != 1 {
			ch.Unresolved = append(ch.Unresolved, fmt.Sprintf("%s direct internal calls=%d", ep.Symbol, len(facts)))
		} else if !facts[0].Resolved {
			ch.Unresolved = append(ch.Unresolved, ep.Symbol+" receiver unresolved")
		} else {
			first := facts[0]
			if info, ok := infos[first.ReceiverType]; ok && changedSet[filepath.ToSlash(info.Path)] {
				affected[ch.Name] = true
			}
			xs := impls[first.ReceiverType]
			// Candidate implementations establish possible impact, not a
			// unique execution path. Keep ambiguity in Unresolved below.
			for _, candidate := range xs {
				if changedSet[filepath.ToSlash(candidate.Path)] {
					affected[ch.Name] = true
				}
			}
			if len(xs) != 1 {
				ch.Unresolved = append(ch.Unresolved, fmt.Sprintf("%s implementations=%d", first.ReceiverType, len(xs)))
			} else {
				service := xs[0]
				ss := service.Symbol + "." + first.Method
				ch.Nodes = append(ch.Nodes, Node{Path: filepath.ToSlash(service.Path), Symbol: ss, Role: "SERVICE", Workspace: "current"})
				if changedSet[filepath.ToSlash(service.Path)] {
					affected[ch.Name] = true
				}
				sf := calls[ss]
				if len(sf) != 1 {
					ch.Unresolved = append(ch.Unresolved, fmt.Sprintf("%s direct internal calls=%d", ss, len(sf)))
				} else if !sf[0].Resolved {
					ch.Unresolved = append(ch.Unresolved, ss+" receiver unresolved")
				} else {
					ms := sf[0].TargetSymbol
					info, ok := infos[ms]
					if !ok {
						ch.Unresolved = append(ch.Unresolved, ms+" declaration unresolved")
					} else {
						ch.Nodes = append(ch.Nodes, Node{Path: filepath.ToSlash(info.Path), Symbol: ms, Role: "MAPPER", Workspace: "current"})
						if changedSet[filepath.ToSlash(info.Path)] {
							affected[ch.Name] = true
						}
						xmlKey, identityOK := mapperXMLIdentity180(rootAbs, info, ms)
						if identityOK {
							if xp, ok := xmlIndex[xmlKey]; ok {
								ch.Nodes = append(ch.Nodes, Node{Path: xp, Symbol: ms, Role: "SQL", Workspace: "current"})
								if changedSet[xp] {
									affected[ch.Name] = true
								}
							} else {
								ch.Unresolved = append(ch.Unresolved, ms+" XML statement unresolved")
							}
						} else {
							ch.Unresolved = append(ch.Unresolved, ms+" XML statement unresolved")
						}
					}
				}
			}
		}
		chains = append(chains, ch)
	}

	if intent.Mode == "CHANGES" {
		filtered := make([]Chain, 0, len(chains))
		for _, ch := range chains {
			// An incomplete path cannot prove that downstream changed code
			// is unrelated. Retain the entry and its gap rather than allow
			// another, complete chain to manufacture AUTO_SINGLE.
			if !affected[ch.Name] && len(ch.Unresolved) > 0 && len(changedSet) > 0 {
				ch.Unresolved = append(ch.Unresolved, "CHANGE_IMPACT_UNRESOLVED: "+ch.Name)
				affected[ch.Name] = true
			}
			if affected[ch.Name] {
				filtered = append(filtered, ch)
			}
		}
		chains = filtered
	}

	sort.Slice(chains, func(i, j int) bool { return chains[i].Name < chains[j].Name })
	for i := range chains {
		chains[i].ID = fmt.Sprintf("C%d", i+1)
	}
	gaps := []string{}
	if len(chains) == 0 {
		gaps = append(gaps, "ENTRYPOINT_NOT_FOUND")
	}
	for _, ch := range chains {
		gaps = append(gaps, ch.Unresolved...)
	}

	after, _, err := fingerprintPaths180(rootAbs, sources)
	if err != nil {
		return Options{}, err
	}
	currentSources, err := sourcePaths180(rootAbs)
	if err != nil {
		return Options{}, err
	}
	if after != before || !sameStringSlice180(currentSources, sources) {
		gaps = append(gaps, "SOURCE_CHANGED_DURING_PREPARE")
	}
	complete := len(gaps) == 0
	return persistPrepared180(runDir, state, intent, rootAbs, sources, before, chains, complete, uniqueStrings180(gaps), runner.Count())
}

func persistPrepared180(runDir string, state runState, intent Intent, root string, paths []string, fp string, chains []Chain, complete bool, gaps []string, processes int) (Options, error) {
	lockCtx, cancel := context.WithTimeout(context.Background(), defaultRunLockWait)
	defer cancel()
	unlock, err := acquireRunLock(lockCtx, runDir)
	if err != nil {
		return Options{}, err
	}
	defer unlock()

	freshRunDir, freshState, err := loadRun(root, state.RunID)
	if err != nil {
		return Options{}, err
	}
	if filepath.Clean(freshRunDir) != filepath.Clean(runDir) {
		return Options{}, fmt.Errorf("REVIEW_PREPARE_RUN_CHANGED")
	}
	if freshState.Cancelled {
		return Options{}, fmt.Errorf("REVIEW_PREPARE_CANCELLED")
	}
	state = freshState

	if chains == nil {
		chains = []Chain{}
	}
	opts := Options{RunID: state.RunID, Chains: chains, DiscoveryComplete: complete, SelectionRequired: len(chains) > 1, Gaps: nonNilStrings180(gaps), ReportPath: state.ReportPath}
	opts.Hash = optionsHash180(root, intent, fp, opts)
	stored := preparedOptions180{SchemaVersion: SchemaVersion, Intent: intent, ProjectRoot: root, SourceFingerprint: fp, SourcePaths: nonNilStrings180(paths), NavigationProcesses: processes, Options: opts}
	b, _ := json.MarshalIndent(stored, "", "  ")
	b = append(b, '\n')
	state.ScopeReady = false
	state.Coverage = "PARTIAL"
	state.SelectedIDs = []string{}
	state.LastError = ""
	if err := writeState(runDir, state); err != nil {
		return Options{}, err
	}
	scopePath := filepath.Join(runDir, "scope.json")
	if err := os.Remove(scopePath); err != nil && !os.IsNotExist(err) {
		return Options{}, fmt.Errorf("REVIEW_SCOPE_RESET_FAILED: %w", err)
	}
	if err := atomicWrite(filepath.Join(runDir, "options.json"), b); err != nil {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_WRITE_FAILED: %w", err)
	}
	if complete && len(chains) <= 1 {
		if len(chains) == 1 {
			state.SelectedIDs = []string{chains[0].ID}
		}
		if err := writeScope180(runDir, state.RunID, opts.Hash, state.SelectedIDs, chainsForIDs180(chains, state.SelectedIDs), root); err != nil {
			return Options{}, err
		}
		state.ScopeReady = true
		state.Coverage = "COMPLETE"
		if err := writeState(runDir, state); err != nil {
			return Options{}, err
		}
	}
	return opts, nil
}

func loadPreparedOptions180(runDir string) (preparedOptions180, error) {
	b, err := os.ReadFile(filepath.Join(runDir, "options.json"))
	if err != nil {
		return preparedOptions180{}, err
	}
	var v preparedOptions180
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return v, fmt.Errorf("REVIEW_OPTIONS_STALE: %w", err)
	}
	if v.SchemaVersion != SchemaVersion || v.Options.RunID == "" {
		return v, fmt.Errorf("REVIEW_OPTIONS_STALE")
	}
	return v, nil
}

var (
	exactControllerTarget180 = regexp.MustCompile("^[A-Za-z_$][A-Za-z0-9_$]*Controller(?:\\.[A-Za-z_$][A-Za-z0-9_$]*)?$")
	controllerTargetInText180 = regexp.MustCompile("[A-Za-z_$][A-Za-z0-9_$]*Controller(?:\\.[A-Za-z_$][A-Za-z0-9_$]*)?")
)

func normalizeIntent180(v Intent) (Intent, error) {
	v.Mode = strings.ToUpper(strings.TrimSpace(v.Mode))
	v.Target = normalizeReviewTarget180(v.Target)
	if v.Mode != "CHANGES" && v.Mode != "CURRENT_IMPLEMENTATION" {
		return Intent{}, fmt.Errorf("REVIEW_PREPARE_INTENT_INVALID: %q", v.Mode)
	}
	return v, nil
}

func normalizeReviewTarget180(raw string) string {
	target := strings.TrimSpace(raw)
	if target == "" || exactControllerTarget180.MatchString(target) {
		return target
	}
	// Paths are already supported by filterEndpoints180 and must never be
	// rewritten just because their basename contains a Controller symbol.
	if strings.ContainsAny(target, "/\\") {
		return target
	}
	matches := controllerTargetInText180.FindAllString(target, -1)
	unique := map[string]bool{}
	for _, match := range matches {
		unique[match] = true
	}
	if len(unique) != 1 {
		// Fail closed on ambiguous prose instead of guessing which entrypoint
		// the user intended.
		return target
	}
	for match := range unique {
		return match
	}
	return target
}

func astGrepPath180(root string) (string, error) {
	if p := strings.TrimSpace(os.Getenv("CODEA_AST_GREP_TEST_PATH")); p != "" {
		return p, nil
	}
	n := "ast-grep"
	if runtime.GOOS == "windows" {
		n = "ast-grep.exe"
	}
	p := filepath.Join(root, ".code-harness", "bin", n)
	if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() {
		return p, nil
	}
	if p, e := exec.LookPath("ast-grep"); e == nil {
		return p, nil
	}
	return "", fmt.Errorf("ast-grep executable not found")
}

func sourcePaths180(root string) ([]string, error) {
	out := []string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".code-harness", "node_modules", "target", "build":
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".java" && ext != ".xml" {
			return nil
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}

func filterExt180(ps []string, ext string) []string {
	out := []string{}
	for _, p := range ps {
		if strings.EqualFold(filepath.Ext(p), ext) {
			out = append(out, p)
		}
	}
	return out
}

func fingerprintPaths180(root string, ps []string) (string, map[string]string, error) {
	h := sha256.New()
	m := map[string]string{}
	for _, p := range ps {
		b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if e != nil {
			return "", nil, e
		}
		s := sha256.Sum256(b)
		x := hex.EncodeToString(s[:])
		m[p] = x
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(x))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), m, nil
}

func sourceSnapshotMatches180(root string, expectedPaths []string, expectedFingerprint string) (bool, error) {
	current, err := sourcePaths180(root)
	if err != nil {
		return false, err
	}
	if !sameStringSlice180(current, expectedPaths) {
		return false, nil
	}
	fresh, _, err := fingerprintPaths180(root, current)
	if err != nil {
		return false, err
	}
	return fresh == expectedFingerprint, nil
}

func sameStringSlice180(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func optionsHash180(root string, intent Intent, fp string, o Options) string {
	v := struct {
		Root        string
		Intent      Intent
		Fingerprint string
		Chains      []Chain
		Complete    bool
		Gaps        []string
	}{filepath.Clean(root), intent, fp, o.Chains, o.DiscoveryComplete, o.Gaps}
	b, _ := json.Marshal(v)
	return bytesSHA256(b)
}

func scanScope180(ps []string) string {
	if len(ps) == 0 {
		return "src"
	}
	x := strings.Split(filepath.ToSlash(ps[0]), "/")[0]
	if x == "" || x == "." {
		return "src"
	}
	return x
}

func filterEndpoints180(in []nav.ControllerEndpointMatch, target string) []nav.ControllerEndpointMatch {
	target = strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	if target == "" {
		return in
	}
	out := []nav.ControllerEndpointMatch{}
	for _, e := range in {
		if e.Symbol == target || strings.HasPrefix(e.Symbol, target+".") || strings.HasSuffix(e.Path, "/"+target) || strings.TrimSuffix(filepath.Base(e.Path), filepath.Ext(e.Path)) == target {
			out = append(out, e)
		}
	}
	return out
}

func uniqueStrings180(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func nonNilStrings180(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func changedSourceFiles180(ctx context.Context, root string) ([]string, error) {
	set := map[string]bool{}
	for _, args := range [][]string{{"diff", "--name-only", "-z", "HEAD"}, {"diff", "--cached", "--name-only", "-z", "HEAD"}, {"ls-files", "--others", "--exclude-standard", "-z"}} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = root
		b, e := cmd.Output()
		if e != nil {
			return nil, e
		}
		for _, line := range strings.Split(string(b), "\x00") {
			p := filepath.ToSlash(line)
			if strings.EqualFold(filepath.Ext(p), ".java") || strings.EqualFold(filepath.Ext(p), ".xml") {
				set[p] = true
			}
		}
	}
	out := []string{}
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func sameProjectPath180(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

type mapperXML180 struct {
	Namespace  string `xml:"namespace"`
	Statements []struct {
		XMLName xml.Name
		ID      string `xml:"id"`
	} `xml:",any"`
}

func indexMapperXML180(root string, ps []string) map[string]string {
	out := map[string]string{}
	for _, p := range ps {
		b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if e != nil {
			continue
		}
		var m mapperXML180
		if xml.Unmarshal(b, &m) != nil {
			continue
		}
		namespace := strings.TrimSpace(m.Namespace)
		if namespace == "" {
			continue
		}
		for _, s := range m.Statements {
			if s.ID != "" {
				out[namespace+"."+s.ID] = p
			}
		}
	}
	return out
}

func mapperXMLIdentity180(root string, info nav.SymbolInfo, symbol string) (string, bool) {
	dot := strings.LastIndex(symbol, ".")
	if dot <= 0 || dot == len(symbol)-1 {
		return "", false
	}
	owner := symbol[:dot]
	method := symbol[dot+1:]
	pkg, ok := javaPackage180(root, info.Path)
	if !ok {
		return "", false
	}
	if pkg != "" {
		owner = pkg + "." + owner
	}
	return owner + "." + method, true
}

func javaPackage180(root, rel string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "package ") {
			continue
		}
		pkg := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, "package ")), ";"))
		if pkg == "" {
			return "", false
		}
		return pkg, true
	}
	return "", true
}

func pathSet180(paths []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p != "" {
			out[p] = true
		}
	}
	return out
}
