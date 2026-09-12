package reviewcontext

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/projectpath"
)

type myBatisDocument170 struct {
	Path      string
	Namespace string
	Data      []byte
	Nodes     []*myBatisNode170
}

type myBatisNode170 struct {
	Document      *myBatisDocument170
	Tag           string
	ID            string
	DatabaseID    string
	ParameterType string
	ResultType    string
	ResultMap     string
	Start         int
	End           int
	Includes      []myBatisInclude170
	Dynamic       bool
	RawSubstitute bool
}

type myBatisInclude170 struct {
	RefID string
	Start int
	End   int
}

type myBatisIndex170 struct {
	Statements map[string][]*myBatisNode170
	Fragments  map[string][]*myBatisNode170
}

var (
	myBatisParamImport170 = regexp.MustCompile(`(?m)(?:^|[;\n])\s*import\s+org\.apache\.ibatis\.annotations\.Param\s*;`)
	myBatisParam170       = regexp.MustCompile(`@Param\s*\(\s*"([A-Za-z_$][A-Za-z0-9_$]*)"\s*\)`)
	myBatisParamFQ170     = regexp.MustCompile(`@org\.apache\.ibatis\.annotations\.Param\s*\(\s*"([A-Za-z_$][A-Za-z0-9_$]*)"\s*\)`)
	myBatisPlaceholder170 = regexp.MustCompile(`(?:#|\$)\{\s*([A-Za-z_$][A-Za-z0-9_$]*)`)
)

// ResolveMapper170 resolves a supplied, already-scoped Java Mapper method to
// MyBatis XML statement/fragment relations under the provided repository root.
// It never derives a Git base, executes SQL, contacts a database, or resolves
// external XML entities.
func ResolveMapper170(ctx context.Context, repoRoot string, source nav.SourceRange170) ([]nav.Relation170, []nav.Issue170, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if source.Ref.Kind != "METHOD" {
		return nil, nil, fmt.Errorf("MYBATIS_SOURCE_INVALID: expected METHOD ref")
	}
	if _, err := nav.ReviewRefKey170(source.Ref); err != nil {
		return nil, nil, fmt.Errorf("MYBATIS_SOURCE_INVALID: %w", err)
	}
	if source.StartLine < 1 || source.EndLine < source.StartLine || source.StartColumn < 1 || source.EndColumn < 1 || (source.StartLine == source.EndLine && source.EndColumn <= source.StartColumn) {
		return nil, nil, fmt.Errorf("MYBATIS_SOURCE_INVALID: invalid source range")
	}
	root, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return nil, nil, err
	}
	root = filepath.Clean(root)
	javaPath, err := myBatisRepoFile170(root, source.Ref.Path)
	if err != nil {
		return nil, nil, err
	}
	javaData, err := os.ReadFile(javaPath)
	if err != nil {
		return nil, nil, fmt.Errorf("MYBATIS_SOURCE_UNAVAILABLE: %w", err)
	}
	javaInfo, err := os.Lstat(javaPath)
	if err != nil {
		return nil, nil, err
	}
	if javaInfo.Mode()&os.ModeSymlink != 0 || !javaInfo.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("MYBATIS_SOURCE_INVALID: Java source is not a regular file")
	}

	docs, err := loadMyBatisDocuments170(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	idx := buildMyBatisIndex170(docs)
	key := myBatisNodeKey170(source.Ref.OwnerFQCN, source.Ref.Name)
	candidates := idx.Statements[key]
	if len(candidates) == 0 {
		relation := nav.Relation170{
			Kind:        "MYBATIS_STATEMENT",
			Resolution:  "UNRESOLVED",
			From:        source.Ref,
			Targets:     []nav.ReviewRef170{},
			Evidence:    []nav.SourceRange170{source},
			Reason:      "MYBATIS_STATEMENT_UNRESOLVED: mapper namespace and statement id were not found",
			Assumptions: []string{},
		}
		relation.ID = myBatisRelationID170(relation)
		return []nav.Relation170{relation}, []nav.Issue170{}, nil
	}

	statementRelation, selected, err := myBatisStatementRelation170(source, candidates)
	if err != nil {
		return nil, nil, err
	}
	relations := []nav.Relation170{statementRelation}
	issues := []nav.Issue170{}
	if selected == nil {
		return relations, issues, nil
	}

	javaNames := myBatisExplicitParams170(javaData, source)
	resolvedText := []byte{}
	stack := map[string]bool{}
	seenEdges := map[string]bool{}
	includeRelations, includeIssues, fragments, err := resolveMyBatisIncludes170(idx, selected, stack, seenEdges)
	if err != nil {
		return nil, nil, err
	}
	relations = append(relations, includeRelations...)
	issues = append(issues, includeIssues...)
	resolvedText = append(resolvedText, myBatisNodeBytes170(selected)...)
	for _, fragment := range fragments {
		resolvedText = append(resolvedText, '\n')
		resolvedText = append(resolvedText, myBatisNodeBytes170(fragment)...)
	}
	if len(javaNames) > 0 {
		missing := myBatisMissingParams170(javaNames, resolvedText)
		if len(missing) > 0 {
			at, rangeErr := myBatisNodeRange170(selected, myBatisNodeRef170(selected, source.Ref.Workspace, source.Ref.Side))
			if rangeErr != nil {
				return nil, nil, rangeErr
			}
			issues = append(issues, nav.Issue170{
				Code:   "MYBATIS_PARAM_CONTRACT_MISMATCH",
				At:     at,
				Detail: "XML references parameters not declared by explicit @Param: " + strings.Join(missing, ", "),
			})
		}
	}

	sort.Slice(relations, func(i, j int) bool {
		if relations[i].Kind != relations[j].Kind {
			return relations[i].Kind < relations[j].Kind
		}
		return relations[i].ID < relations[j].ID
	})
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].At.Ref.Path != issues[j].At.Ref.Path {
			return issues[i].At.Ref.Path < issues[j].At.Ref.Path
		}
		if issues[i].At.StartLine != issues[j].At.StartLine {
			return issues[i].At.StartLine < issues[j].At.StartLine
		}
		if issues[i].At.StartColumn != issues[j].At.StartColumn {
			return issues[i].At.StartColumn < issues[j].At.StartColumn
		}
		return issues[i].Code < issues[j].Code
	})
	return relations, issues, nil
}

func loadMyBatisDocuments170(ctx context.Context, root string) ([]*myBatisDocument170, error) {
	resourceRoot := filepath.Join(root, "src", "main", "resources")
	info, err := os.Stat(resourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []*myBatisDocument170{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []*myBatisDocument170{}, nil
	}
	paths := []string{}
	err = filepath.WalkDir(resourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xml") {
			return nil
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if entryInfo.Mode().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	docs := make([]*myBatisDocument170, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if !bytes.Contains(data, []byte("<mapper")) {
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		doc, err := parseMyBatisDocument170(filepath.ToSlash(rel), data)
		if err != nil {
			return nil, err
		}
		if doc.Namespace != "" {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func parseMyBatisDocument170(path string, data []byte) (*myBatisDocument170, error) {
	doc := &myBatisDocument170{Path: path, Data: data, Nodes: []*myBatisNode170{}}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	depth := 0
	mapperDepth := -1
	var active *myBatisNode170
	activeDepth := -1
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("MYBATIS_XML_INVALID: %s: %w", path, err)
		}
		switch value := token.(type) {
		case xml.Directive:
			directive := strings.TrimSpace(string(value))
			if strings.HasPrefix(strings.ToUpper(directive), "DOCTYPE") && !myBatisKnownMapperDTD170(directive) {
				return nil, fmt.Errorf("MYBATIS_XML_DTD_UNSUPPORTED: %s", path)
			}
		case xml.StartElement:
			start := myBatisStartOffset170(data, int(decoder.InputOffset()))
			if value.Name.Local == "mapper" && mapperDepth < 0 {
				doc.Namespace = strings.TrimSpace(myBatisAttr170(value, "namespace"))
				mapperDepth = depth
			} else if mapperDepth >= 0 && depth == mapperDepth+1 && myBatisNodeTag170(value.Name.Local) {
				active = &myBatisNode170{
					Document:      doc,
					Tag:           value.Name.Local,
					ID:            strings.TrimSpace(myBatisAttr170(value, "id")),
					DatabaseID:    strings.TrimSpace(myBatisAttr170(value, "databaseId")),
					ParameterType: strings.TrimSpace(myBatisAttr170(value, "parameterType")),
					ResultType:    strings.TrimSpace(myBatisAttr170(value, "resultType")),
					ResultMap:     strings.TrimSpace(myBatisAttr170(value, "resultMap")),
					Start:         start,
					Includes:      []myBatisInclude170{},
				}
				activeDepth = depth
				doc.Nodes = append(doc.Nodes, active)
			} else if active != nil {
				if value.Name.Local == "include" {
					active.Includes = append(active.Includes, myBatisInclude170{RefID: strings.TrimSpace(myBatisAttr170(value, "refid")), Start: start, End: int(decoder.InputOffset())})
				}
				if myBatisDynamicTag170(value.Name.Local) {
					active.Dynamic = true
				}
			}
			depth++
		case xml.EndElement:
			depth--
			if active != nil && depth == activeDepth && value.Name.Local == active.Tag {
				active.End = int(decoder.InputOffset())
				if active.End > active.Start && active.End <= len(data) {
					active.RawSubstitute = bytes.Contains(data[active.Start:active.End], []byte("${"))
				}
				active = nil
				activeDepth = -1
			}
		}
	}
	return doc, nil
}

func buildMyBatisIndex170(docs []*myBatisDocument170) myBatisIndex170 {
	idx := myBatisIndex170{Statements: map[string][]*myBatisNode170{}, Fragments: map[string][]*myBatisNode170{}}
	for _, doc := range docs {
		for _, node := range doc.Nodes {
			if node.ID == "" || doc.Namespace == "" {
				continue
			}
			key := myBatisNodeKey170(doc.Namespace, node.ID)
			if node.Tag == "sql" {
				idx.Fragments[key] = append(idx.Fragments[key], node)
			} else {
				idx.Statements[key] = append(idx.Statements[key], node)
			}
		}
	}
	for _, values := range idx.Statements {
		myBatisSortNodes170(values)
	}
	for _, values := range idx.Fragments {
		myBatisSortNodes170(values)
	}
	return idx
}

func myBatisStatementRelation170(source nav.SourceRange170, candidates []*myBatisNode170) (nav.Relation170, *myBatisNode170, error) {
	relation := nav.Relation170{
		Kind:        "MYBATIS_STATEMENT",
		From:        source.Ref,
		Targets:     []nav.ReviewRef170{},
		Evidence:    []nav.SourceRange170{source},
		Assumptions: []string{},
	}
	hasDatabaseID := false
	for _, candidate := range candidates {
		ref := myBatisNodeRef170(candidate, source.Ref.Workspace, source.Ref.Side)
		relation.Targets = appendUniqueMyBatisRef170(relation.Targets, ref)
		rangeValue, err := myBatisNodeRange170(candidate, ref)
		if err != nil {
			return nav.Relation170{}, nil, err
		}
		relation.Evidence = append(relation.Evidence, rangeValue)
		if candidate.DatabaseID != "" {
			hasDatabaseID = true
			relation.Assumptions = append(relation.Assumptions, "mybatis.databaseId="+candidate.DatabaseID)
		}
	}
	if hasDatabaseID {
		relation.Resolution = "CONDITIONAL"
		relation.Reason = "MYBATIS_DATABASE_ID_UNRESOLVED: databaseId configuration unavailable"
		relation.Assumptions = uniqueSortedMyBatis170(relation.Assumptions)
		relation.ID = myBatisRelationID170(relation)
		return relation, nil, nil
	}
	if len(candidates) != 1 {
		relation.Resolution = "AMBIGUOUS"
		relation.Reason = "MYBATIS_STATEMENT_AMBIGUOUS: multiple mapper statements share namespace and id"
		relation.ID = myBatisRelationID170(relation)
		return relation, nil, nil
	}
	selected := candidates[0]
	relation.Resolution = "EXACT"
	relation.Targets = []nav.ReviewRef170{myBatisNodeRef170(selected, source.Ref.Workspace, source.Ref.Side)}
	relation.Assumptions = myBatisStatementAssumptions170(selected)
	relation.ID = myBatisRelationID170(relation)
	return relation, selected, nil
}

func resolveMyBatisIncludes170(idx myBatisIndex170, node *myBatisNode170, stack, seenEdges map[string]bool) ([]nav.Relation170, []nav.Issue170, []*myBatisNode170, error) {
	workspace := "current"
	side := "CURRENT"
	from := myBatisNodeRef170(node, workspace, side)
	nodeKey := myBatisNodeIdentity170(node)
	stack[nodeKey] = true
	defer delete(stack, nodeKey)

	relations := []nav.Relation170{}
	issues := []nav.Issue170{}
	fragments := []*myBatisNode170{}
	for _, include := range node.Includes {
		namespace, id, ok := myBatisIncludeTarget170(node.Document.Namespace, include.RefID)
		includeRange := myBatisOffsetRange170(node.Document, include.Start, include.End, from)
		relation := nav.Relation170{Kind: "SQL_INCLUDE", From: from, Targets: []nav.ReviewRef170{}, Evidence: []nav.SourceRange170{includeRange}, Assumptions: []string{}}
		if !ok {
			relation.Resolution = "UNRESOLVED"
			relation.Reason = "SQL_INCLUDE_UNRESOLVED: refid is not a static namespace/id reference"
			relation.ID = myBatisRelationID170(relation)
			relations = append(relations, relation)
			issues = append(issues, nav.Issue170{Code: "SQL_INCLUDE_UNRESOLVED", At: includeRange, Detail: relation.Reason})
			continue
		}
		candidates := idx.Fragments[myBatisNodeKey170(namespace, id)]
		if len(candidates) == 0 {
			relation.Resolution = "UNRESOLVED"
			relation.Reason = "SQL_INCLUDE_UNRESOLVED: referenced sql fragment was not found"
			relation.ID = myBatisRelationID170(relation)
			relations = append(relations, relation)
			issues = append(issues, nav.Issue170{Code: "SQL_INCLUDE_UNRESOLVED", At: includeRange, Detail: relation.Reason})
			continue
		}
		hasDatabaseID := false
		for _, candidate := range candidates {
			ref := myBatisNodeRef170(candidate, workspace, side)
			relation.Targets = appendUniqueMyBatisRef170(relation.Targets, ref)
			if candidate.DatabaseID != "" {
				hasDatabaseID = true
			}
		}
		if hasDatabaseID {
			relation.Resolution = "CONDITIONAL"
			relation.Reason = "MYBATIS_DATABASE_ID_UNRESOLVED: sql fragment databaseId configuration unavailable"
			relation.ID = myBatisRelationID170(relation)
			relations = append(relations, relation)
			continue
		}
		if len(candidates) != 1 {
			relation.Resolution = "AMBIGUOUS"
			relation.Reason = "SQL_INCLUDE_UNRESOLVED: multiple sql fragments share namespace and id"
			relation.ID = myBatisRelationID170(relation)
			relations = append(relations, relation)
			issues = append(issues, nav.Issue170{Code: "SQL_INCLUDE_UNRESOLVED", At: includeRange, Detail: relation.Reason})
			continue
		}
		target := candidates[0]
		targetRef := myBatisNodeRef170(target, workspace, side)
		targetRange, err := myBatisNodeRange170(target, targetRef)
		if err != nil {
			return nil, nil, nil, err
		}
		relation.Targets = []nav.ReviewRef170{targetRef}
		relation.Evidence = append(relation.Evidence, targetRange)
		if stack[myBatisNodeIdentity170(target)] {
			relation.Resolution = "UNRESOLVED"
			relation.Reason = "SQL_INCLUDE_CYCLE: sql fragment include graph is cyclic"
			relation.ID = myBatisRelationID170(relation)
			relations = append(relations, relation)
			issues = append(issues, nav.Issue170{Code: "SQL_INCLUDE_CYCLE", At: includeRange, Detail: relation.Reason})
			continue
		}
		relation.Resolution = "EXACT"
		relation.ID = myBatisRelationID170(relation)
		edgeKey := relation.ID
		if !seenEdges[edgeKey] {
			seenEdges[edgeKey] = true
			relations = append(relations, relation)
		}
		fragments = appendUniqueMyBatisNode170(fragments, target)
		nestedRelations, nestedIssues, nestedFragments, err := resolveMyBatisIncludes170(idx, target, stack, seenEdges)
		if err != nil {
			return nil, nil, nil, err
		}
		relations = append(relations, nestedRelations...)
		issues = append(issues, nestedIssues...)
		for _, fragment := range nestedFragments {
			fragments = appendUniqueMyBatisNode170(fragments, fragment)
		}
	}
	return relations, issues, fragments, nil
}

func myBatisExplicitParams170(javaData []byte, source nav.SourceRange170) map[string]bool {
	text := string(javaData)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	start, end := source.StartLine-1, source.EndLine
	if start < 0 || start >= len(lines) {
		return nil
	}
	if end > len(lines) {
		end = len(lines)
	}
	snippet := strings.Join(lines[start:end], "\n")
	if !myBatisParamImport170.Match(javaData) && !strings.Contains(snippet, "@org.apache.ibatis.annotations.Param") {
		return nil
	}
	out := map[string]bool{}
	for _, re := range []*regexp.Regexp{myBatisParam170, myBatisParamFQ170} {
		for _, match := range re.FindAllStringSubmatch(snippet, -1) {
			if len(match) == 2 {
				out[match[1]] = true
			}
		}
	}
	return out
}

func myBatisMissingParams170(known map[string]bool, data []byte) []string {
	missing := map[string]bool{}
	for _, match := range myBatisPlaceholder170.FindAllSubmatch(data, -1) {
		if len(match) != 2 {
			continue
		}
		name := string(match[1])
		if known[name] || name == "_parameter" || name == "_databaseId" || strings.HasPrefix(name, "param") || strings.HasPrefix(name, "arg") {
			continue
		}
		missing[name] = true
	}
	out := make([]string, 0, len(missing))
	for name := range missing {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func myBatisNodeBytes170(node *myBatisNode170) []byte {
	if node == nil || node.Document == nil || node.Start < 0 || node.End <= node.Start || node.End > len(node.Document.Data) {
		return nil
	}
	return node.Document.Data[node.Start:node.End]
}

func myBatisStatementAssumptions170(node *myBatisNode170) []string {
	out := []string{}
	if node.Dynamic {
		out = append(out, "mybatis.dynamicSql=true")
	}
	if node.RawSubstitute {
		out = append(out, "mybatis.rawSubstitution=true")
	}
	if node.ParameterType != "" {
		out = append(out, "mybatis.parameterType="+node.ParameterType)
	}
	if node.ResultType != "" {
		out = append(out, "mybatis.resultType="+node.ResultType)
	}
	if node.ResultMap != "" {
		out = append(out, "mybatis.resultMap="+node.ResultMap)
	}
	return uniqueSortedMyBatis170(out)
}

func myBatisNodeRef170(node *myBatisNode170, workspace, side string) nav.ReviewRef170 {
	kind := "STATEMENT"
	if node.Tag == "sql" {
		kind = "SQL_FRAGMENT"
	}
	return nav.ReviewRef170{
		Workspace:      workspace,
		Path:           node.Document.Path,
		Side:           side,
		Kind:           kind,
		OwnerFQCN:      node.Document.Namespace,
		Name:           node.ID,
		ParameterTypes: []string{},
	}
}

func myBatisNodeRange170(node *myBatisNode170, ref nav.ReviewRef170) (nav.SourceRange170, error) {
	if node == nil || node.Document == nil || node.Start < 0 || node.End <= node.Start || node.End > len(node.Document.Data) {
		return nav.SourceRange170{}, fmt.Errorf("MYBATIS_XML_RANGE_INVALID")
	}
	return myBatisOffsetRange170(node.Document, node.Start, node.End, ref), nil
}

func myBatisOffsetRange170(doc *myBatisDocument170, start, end int, ref nav.ReviewRef170) nav.SourceRange170 {
	startLine, startCol := myBatisLineColumn170(doc.Data, start)
	endLine, endCol := myBatisLineColumn170(doc.Data, end)
	if endLine == startLine && endCol <= startCol {
		endCol = startCol + 1
	}
	return nav.SourceRange170{Ref: ref, StartLine: startLine, EndLine: endLine, StartColumn: startCol, EndColumn: endCol}
}

func myBatisLineColumn170(data []byte, offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	line := 1
	lineStart := 0
	for i, b := range data[:offset] {
		if b == '\n' {
			line++
			lineStart = i + 1
		}
	}
	column := utf8.RuneCount(data[lineStart:offset]) + 1
	return line, column
}

func myBatisRelationID170(relation nav.Relation170) string {
	parts := []string{relation.Kind, relation.Resolution, relation.Reason}
	if key, err := nav.ReviewRefKey170(relation.From); err == nil {
		parts = append(parts, key)
	}
	targets := []string{}
	for _, target := range relation.Targets {
		if key, err := nav.ReviewRefKey170(target); err == nil {
			targets = append(targets, key)
		}
	}
	sort.Strings(targets)
	parts = append(parts, targets...)
	for _, evidence := range relation.Evidence {
		key, _ := nav.ReviewRefKey170(evidence.Ref)
		parts = append(parts, fmt.Sprintf("%s:%d:%d:%d:%d", key, evidence.StartLine, evidence.StartColumn, evidence.EndLine, evidence.EndColumn))
	}
	assumptions := append([]string{}, relation.Assumptions...)
	sort.Strings(assumptions)
	parts = append(parts, assumptions...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return strings.ToLower(relation.Kind) + ":" + hex.EncodeToString(sum[:])
}

func myBatisRepoFile170(root, raw string) (string, error) {
	normalized, ok := projectpath.Normalize(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	if !ok {
		return "", fmt.Errorf("MYBATIS_SOURCE_INVALID: unsafe path %q", raw)
	}
	candidate := filepath.Join(root, filepath.FromSlash(normalized))
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(filepath.ToSlash(rel), "../") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("MYBATIS_SOURCE_INVALID: path escaped repository root")
	}
	return filepath.Clean(abs), nil
}

func myBatisIncludeTarget170(currentNamespace, refid string) (string, string, bool) {
	refid = strings.TrimSpace(refid)
	if refid == "" || strings.ContainsAny(refid, "${}# ") {
		return "", "", false
	}
	if dot := strings.LastIndexByte(refid, '.'); dot >= 0 {
		if dot == 0 || dot == len(refid)-1 {
			return "", "", false
		}
		return refid[:dot], refid[dot+1:], true
	}
	if currentNamespace == "" {
		return "", "", false
	}
	return currentNamespace, refid, true
}

func myBatisNodeKey170(namespace, id string) string {
	return strings.TrimSpace(namespace) + "\x00" + strings.TrimSpace(id)
}

func myBatisNodeIdentity170(node *myBatisNode170) string {
	if node == nil || node.Document == nil {
		return ""
	}
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", node.Document.Path, node.Document.Namespace, node.Tag, node.ID, node.Start)
}

func myBatisKnownMapperDTD170(directive string) bool {
	normalized := strings.Join(strings.Fields(directive), " ")
	return strings.Contains(normalized, `-//mybatis.org//DTD Mapper 3.0//EN`) &&
		(strings.Contains(normalized, `http://mybatis.org/dtd/mybatis-3-mapper.dtd`) ||
			strings.Contains(normalized, `https://mybatis.org/dtd/mybatis-3-mapper.dtd`))
}

func myBatisNodeTag170(tag string) bool {
	switch tag {
	case "select", "insert", "update", "delete", "sql":
		return true
	default:
		return false
	}
}

func myBatisDynamicTag170(tag string) bool {
	switch tag {
	case "if", "choose", "when", "otherwise", "foreach", "trim", "where", "set", "bind":
		return true
	default:
		return false
	}
}

func myBatisAttr170(element xml.StartElement, name string) string {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func myBatisStartOffset170(data []byte, end int) int {
	if end < 0 {
		return 0
	}
	if end > len(data) {
		end = len(data)
	}
	if idx := bytes.LastIndexByte(data[:end], '<'); idx >= 0 {
		return idx
	}
	return 0
}

func myBatisSortNodes170(nodes []*myBatisNode170) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Document.Path != nodes[j].Document.Path {
			return nodes[i].Document.Path < nodes[j].Document.Path
		}
		if nodes[i].Start != nodes[j].Start {
			return nodes[i].Start < nodes[j].Start
		}
		return nodes[i].DatabaseID < nodes[j].DatabaseID
	})
}

func appendUniqueMyBatisRef170(values []nav.ReviewRef170, ref nav.ReviewRef170) []nav.ReviewRef170 {
	key, err := nav.ReviewRefKey170(ref)
	if err != nil {
		return values
	}
	for _, value := range values {
		valueKey, valueErr := nav.ReviewRefKey170(value)
		if valueErr == nil && valueKey == key {
			return values
		}
	}
	return append(values, ref)
}

func appendUniqueMyBatisNode170(values []*myBatisNode170, node *myBatisNode170) []*myBatisNode170 {
	key := myBatisNodeIdentity170(node)
	for _, value := range values {
		if myBatisNodeIdentity170(value) == key {
			return values
		}
	}
	return append(values, node)
}

func uniqueSortedMyBatis170(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
