from pathlib import Path

p=Path('.code-harness/tools-runtime/internal/reviewcontext/build_170.go')
s=p.read_text()
s=s.replace('''type budgetState170 struct {
	inputFiles map[string]bool
	files      map[string]bool
	candidates int
	blocked    map[string]bool
}''','''type budgetState170 struct {
	inputFiles  map[string]bool
	files       map[string]bool
	candidates  int
	sourceBytes int
	blocked     map[string]bool
}

type sourceSizer170 interface {
	SourceBytes(context.Context, nav.ReviewRef170) (int, error)
}''')
s=s.replace('newFiles := map[string]bool{}','newFiles := map[string]nav.ReviewRef170{}')
s=s.replace('newFiles[key] = true\n\t\t\t}','newFiles[key] = target\n\t\t\t}',1)
old='''		for _, evidence := range rel.Evidence {
			key := fileKey170(evidence.Ref)
			if key != "" && !state.inputFiles[key] && !state.files[key] {
				newFiles[key] = true
			}
		}'''
new='''		for _, evidence := range rel.Evidence {
			key := fileKey170(evidence.Ref)
			if key != "" && !state.inputFiles[key] && !state.files[key] {
				newFiles[key] = evidence.Ref
			}
		}'''
assert old in s
s=s.replace(old,new)
old='''		if budget.MaxFiles > 0 && len(state.files)+len(newFiles) > budget.MaxFiles {
			addBudgetIssue(rel.Kind, firstEvidence170(rel))
			return nil
		}
		for key := range newFiles {
			state.files[key] = true
		}'''
new='''		if budget.MaxFiles > 0 && len(state.files)+len(newFiles) > budget.MaxFiles {
			addBudgetIssue(rel.Kind, firstEvidence170(rel))
			return nil
		}
		additionalBytes := 0
		if sizer, ok := resolver.(sourceSizer170); ok {
			for _, ref := range newFiles {
				size, err := sizer.SourceBytes(ctx, ref)
				if err != nil { return fmt.Errorf("REVIEW_CONTEXT_SOURCE_SIZE_FAILED: %w", err) }
				if size > 0 { additionalBytes += size }
			}
		}
		if budget.MaxSourceBytes > 0 && state.sourceBytes+additionalBytes > budget.MaxSourceBytes {
			addBudgetIssue(rel.Kind, firstEvidence170(rel))
			return nil
		}
		for key := range newFiles { state.files[key] = true }
		state.sourceBytes += additionalBytes'''
assert old in s
s=s.replace(old,new)
old='''			if relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth < budget.MaxDownstreamDepth {
				for _, target := range relation.Targets {
					if target.Side == "CURRENT" && target.Kind == "METHOD" {
						queue = append(queue, queue170{ref: target, downDepth: item.downDepth + 1, upDepth: item.upDepth})
					}
				}
			}'''
new='''			if relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth < budget.MaxDownstreamDepth {
				for _, target := range relation.Targets {
					if target.Side == "CURRENT" && target.Kind == "METHOD" {
						queue = append(queue, queue170{ref: target, downDepth: item.downDepth + 1, upDepth: item.upDepth})
					}
				}
			} else if relation.Resolution == "EXACT" && relation.Kind == "JAVA_CALL" && item.downDepth >= budget.MaxDownstreamDepth {
				addIssue(nav.Issue170{Code:"RECURSION_BOUNDARY", At:firstEvidence170(relation), Detail:"downstream review context depth limit reached"})
			}'''
assert old in s
s=s.replace(old,new)
old='''			if relation.Resolution == "EXACT" && item.upDepth < budget.MaxUpstreamDepth && relation.From.Side == "CURRENT" && relation.From.Kind == "METHOD" {
				queue = append(queue, queue170{ref: relation.From, downDepth: item.downDepth, upDepth: item.upDepth + 1})
			}'''
new='''			if relation.Resolution == "EXACT" && item.upDepth < budget.MaxUpstreamDepth && relation.From.Side == "CURRENT" && relation.From.Kind == "METHOD" {
				queue = append(queue, queue170{ref: relation.From, downDepth: item.downDepth, upDepth: item.upDepth + 1})
			} else if relation.Resolution == "EXACT" && item.upDepth >= budget.MaxUpstreamDepth {
				addIssue(nav.Issue170{Code:"RECURSION_BOUNDARY", At:firstEvidence170(relation), Detail:"upstream review context depth limit reached"})
			}'''
assert old in s
s=s.replace(old,new)
s=s.replace('SourceBytes: 0, ElapsedMillis:', 'SourceBytes: state.sourceBytes, ElapsedMillis:')
p.write_text(s)

p=Path('.code-harness/tools-runtime/cmd/codea-dcep-tools/review_context_command_170.go')
s=p.read_text()
s=s.replace('analysisruntime "codea-harness-tools/internal/analysis"', 'analysisruntime "codea-harness-tools/internal/analysis"\n\t"codea-harness-tools/internal/changeset"')
start=s.index('func runReviewContext170(args []string) error {')
end=s.index('\ntype runtimeResolver170 struct {',start)
replacement=r'''func reviewContextAuthority170(runID, phase string) (string, bool, error) {
	phase = strings.ToUpper(strings.TrimSpace(phase))
	switch phase {
	case "DISCOVERY":
		return filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-set.json")), false, nil
	case "RULES":
		return filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")), true, nil
	default:
		return "", false, fmt.Errorf("REVIEW_CONTEXT_PHASE_INVALID: %q", phase)
	}
}

func reviewContextArtifactPath170(runID, phase string) string {
	name := "review-call-context.json"
	if strings.EqualFold(strings.TrimSpace(phase), "RULES") { name = "review-rule-context.json" }
	return filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", name))
}

func loadFreshChangeSet170(runID string) (changeset.Snapshot, error) {
	path, _, err := reviewContextAuthority170(runID, "DISCOVERY")
	if err != nil { return changeset.Snapshot{}, err }
	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil { return changeset.Snapshot{}, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_UNAVAILABLE: %w", err) }
	var snap changeset.Snapshot
	dec := json.NewDecoder(bytes.NewReader(raw)); dec.DisallowUnknownFields()
	if err := dec.Decode(&snap); err != nil { return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_INVALID: %w", err) }
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) { return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_INVALID: trailing JSON") }
	fresh, err := changeset.Compute(".", snap.RequestedBaseRef, snap.IncludeWorkingTree)
	if err != nil { return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_STALE: %w", err) }
	if fresh.SnapshotSHA256 != snap.SnapshotSHA256 || fresh.HeadCommit != snap.HeadCommit || fresh.MergeBase != snap.MergeBase || fresh.ResolvedBaseCommit != snap.ResolvedBaseCommit {
		return snap, fmt.Errorf("REVIEW_CONTEXT_CHANGESET_STALE: current ChangeSet identity changed")
	}
	return snap,nil
}

func currentJavaChangePaths170(snap changeset.Snapshot) []string {
	out:=[]string{}; seen:=map[string]bool{}
	for _, file := range snap.Files {
		p:=filepath.ToSlash(filepath.Clean(file.Path)); if !strings.HasSuffix(strings.ToLower(p),".java") { continue }
		if _,err:=os.Stat(filepath.FromSlash(p)); err!=nil { continue }
		key:=strings.ToLower(p); if seen[key] { continue }; seen[key]=true; out=append(out,p)
	}
	sort.Strings(out); return out
}

func resourcesForSeeds170(ctx context.Context,n nav.Navigator,seeds []nav.ReviewRef170)([]nav.SourceRange170,error){
	paths:=[]string{}; seen:=map[string]bool{}; wanted:=map[string]bool{}
	for _,seed:=range seeds{key,err:=nav.ReviewRefKey170(seed);if err!=nil{return nil,err};wanted[key]=true;p:=filepath.ToSlash(seed.Path);pk:=strings.ToLower(p);if !seen[pk]{seen[pk]=true;paths=append(paths,p)}}
	refs,ranges,err:=n.DiscoverMethods170(ctx,paths);if err!=nil{return nil,err};out:=[]nav.SourceRange170{}
	for i,ref:=range refs{key,_:=nav.ReviewRefKey170(ref);if wanted[key]&&i<len(ranges){out=append(out,ranges[i])}}
	return out,nil
}

func runReviewContext170(args []string) error {
	fs:=flag.NewFlagSet("review context",flag.ContinueOnError); inputPath:=fs.String("input","","same-run review context request under .code-harness/runs/<runId>/requests")
	if err:=fs.Parse(args);err!=nil{return err};if fs.NArg()!=0||strings.TrimSpace(*inputPath)==""{return errors.New("review context requires --input")}
	runID,cleanInput,err:=validateAnalysisRequestPath153(*inputPath);if err!=nil{return errors.New("review context input must be under .code-harness/runs/<runId>/requests")};if err:=verifyReviewTransportPath153(runID,cleanInput);err!=nil{return err}
	raw,err:=os.ReadFile(cleanInput);if err!=nil{return fmt.Errorf("REVIEW_CONTEXT_REQUEST_READ_FAILED: %w",err)};req,err:=decodeReviewContextRequest170(runID,raw);if err!=nil{return err}
	state,err:=reviewprogress.Read(".",runID);if err!=nil{return err};if err:=validateReviewContextProgress170(state,req.Phase);err!=nil{return err}
	resolver,err:=newRuntimeResolver170(".");if err!=nil{return err};buildInput:=reviewcontext.BuildInput170{RunID:runID,Phase:req.Phase,Budget:reviewcontext.DefaultBudget170()};var unitsPath,dispatchPath string
	if req.Phase=="DISCOVERY"{
		snap,err:=loadFreshChangeSet170(runID);if err!=nil{return err};methods,ranges,err:=resolver.navigators["current"].DiscoverMethods170(context.Background(),currentJavaChangePaths170(snap));if err!=nil{return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_METHODS_FAILED: %w",err)};buildInput.Seeds=methods;buildInput.Resources=ranges
	}else{
		if _,err:=os.Stat(filepath.FromSlash(reviewContextArtifactPath170(runID,"DISCOVERY")));err!=nil{return fmt.Errorf("REVIEW_CONTEXT_DISCOVERY_UNAVAILABLE: %w",err)}
		analysisPath,_,_:=reviewContextAuthority170(runID,"RULES");analysisValue,_,err:=analysisruntime.LoadCertified(".",analysisPath);if err!=nil{return fmt.Errorf("REVIEW_CONTEXT_ANALYSIS_UNAVAILABLE: %w",err)};buildInput.Seeds=seedsFromAnalysis170(analysisValue);buildInput.Resources,err=resourcesForSeeds170(context.Background(),resolver.navigators["current"],buildInput.Seeds);if err!=nil{return fmt.Errorf("REVIEW_CONTEXT_RULE_RESOURCES_FAILED: %w",err)}
		unitsPath=filepath.ToSlash(filepath.Join(".code-harness","runs",runID,"analysis","review-units.json"));dispatchPath=filepath.ToSlash(filepath.Join(".code-harness","runs",runID,"analysis","rule-dispatch.json"));units,err:=reviewunit.Load(reviewunit.BuildInput{RunID:runID,CertifiedRunID:runID,RepoRoot:"."});if err!=nil{return fmt.Errorf("REVIEW_CONTEXT_RULES_UNITS_UNAVAILABLE: %w",err)};dispatch,err:=loadRuleDispatch170(dispatchPath);if err!=nil{return err};if dispatch.RunID!=runID||dispatch.ReviewUnitsSHA256!=units.SHA256{return fmt.Errorf("REVIEW_CONTEXT_RULES_DISPATCH_STALE")};buildInput.Needs=needsFromDispatch170(units,dispatch,analysisValue)
	}
	built,err:=reviewcontext.Build170(context.Background(),buildInput,resolver);if err!=nil{return err};if err:=reviewcontext.VerifyRelations170(context.Background(),built,buildInput,resolver);err!=nil{return err};encoded,err:=json.MarshalIndent(built,"","  ");if err!=nil{return fmt.Errorf("REVIEW_CONTEXT_ENCODE_FAILED: %w",err)};encoded=append(encoded,'\n');artifactPath:=reviewContextArtifactPath170(runID,req.Phase);if err:=atomicReviewWrite153(filepath.FromSlash(artifactPath),encoded);err!=nil{return fmt.Errorf("REVIEW_CONTEXT_WRITE_FAILED: %w",err)}
	if req.Phase=="RULES"{if err:=advanceReviewProgressStage164(runID,reviewprogress.StageReviewPlanning,unitsPath,dispatchPath,artifactPath);err!=nil{return err}}
	return writeJSONAndStatus(map[string]any{"status":"READY","runId":runID,"phase":req.Phase,"artifactPath":artifactPath,"context":built},true)
}
'''
s=s[:start]+replacement+s[end:]
old='''func (r *runtimeResolver170) Callers(context.Context, nav.ReviewRef170) ([]nav.Relation170, error) {
	// T2 exposes exact forward call resolution. Upstream expansion remains
	// fail-closed until a caller candidate can be re-proven as an exact forward
	// edge; never manufacture a caller relation from name-only legacy search.
	return []nav.Relation170{}, nil
}'''
new='''func (r *runtimeResolver170) Callers(ctx context.Context, ref nav.ReviewRef170) ([]nav.Relation170, error) {
	n, err := r.navigator(ref); if err != nil { return nil, err }
	return n.ExactCallers170(ctx, ref)
}
func (r *runtimeResolver170) SourceBytes(_ context.Context, ref nav.ReviewRef170) (int, error) {
	n, err := r.navigator(ref); if err != nil { return 0, err }
	root, err := filepath.Abs(n.RepoRoot); if err != nil { return 0, err }
	candidate, err := filepath.Abs(filepath.Join(root,filepath.FromSlash(ref.Path))); if err != nil { return 0, err }
	rel, err := filepath.Rel(root,candidate); if err != nil || rel==".." || strings.HasPrefix(rel,".."+string(filepath.Separator)) || filepath.IsAbs(rel) { return 0, fmt.Errorf("REVIEW_CONTEXT_SOURCE_PATH_INVALID: %s",ref.Path) }
	info,err:=os.Lstat(candidate);if err!=nil{return 0,err};if info.Mode()&os.ModeSymlink!=0||!info.Mode().IsRegular(){return 0,fmt.Errorf("REVIEW_CONTEXT_SOURCE_PATH_INVALID: %s",ref.Path)}
	return int(info.Size()),nil
}'''
assert old in s
s=s.replace(old,new)
p.write_text(s)

p=Path('.code-harness/tools-runtime/cmd/codea-dcep-tools/review_context_entry_170.go')
p.write_text(p.read_text().replace('"review-context-discovery.json"','"review-call-context.json"'))

p=Path('.github/workflows/task170-t5-context.yml')
s=p.read_text();needle="      - '.code-harness/tools-runtime/internal/reviewcontext/**'\n"
if "internal/nav/**" not in s:s=s.replace(needle,needle+"      - '.code-harness/tools-runtime/internal/nav/**'\n")
p.write_text(s)
