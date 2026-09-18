package reviewrun

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

func findingReadsForChains180(root string, chains []Chain) ([]ReadRef, error) {
	refs := []ReadRef{}
	seen := map[string]bool{}
	for _, chain := range chains {
		for _, node := range chain.Nodes {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(node.Path)))
			if err != nil {
				return nil, fmt.Errorf("REVIEW_FINDING_SCOPE_READ_FAILED: %s: %w", node.Path, err)
			}
			start, end, err := findingNodeRange180(data, node)
			if err != nil {
				return nil, fmt.Errorf("REVIEW_FINDING_SCOPE_UNRESOLVED: %s %s: %w", node.Path, node.Symbol, err)
			}
			ref := ReadRef{Path: node.Path, SHA256: bytesSHA256(data), StartLine: start, EndLine: end}
			key := readKey(ref)
			if !seen[key] {
				seen[key] = true
				refs = append(refs, ref)
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Path != refs[j].Path {
			return refs[i].Path < refs[j].Path
		}
		if refs[i].StartLine != refs[j].StartLine {
			return refs[i].StartLine < refs[j].StartLine
		}
		return refs[i].EndLine < refs[j].EndLine
	})
	return refs, nil
}

func findingNodeRange180(data []byte, node Node) (int, int, error) {
	if strings.EqualFold(node.Role, "SQL") {
		return xmlStatementRange180(data, node.Symbol)
	}
	return javaMethodRange180(data, node.Symbol)
}

func javaMethodRange180(data []byte, symbol string) (int, int, error) {
	dot := strings.LastIndex(symbol, ".")
	if dot < 0 || dot == len(symbol)-1 {
		return 0, 0, fmt.Errorf("method symbol required")
	}
	method := symbol[dot+1:]
	text := string(data)
	lines := strings.Split(text, "\n")
	type candidate struct {
		line int
		col  int
	}
	candidates := []candidate{}
	needle := method + "("
	for i, line := range lines {
		for from := 0; from < len(line); {
			rel := strings.Index(line[from:], needle)
			if rel < 0 {
				break
			}
			idx := from + rel
			from = idx + len(needle)
			if idx > 0 {
				prev := rune(line[idx-1])
				if prev == '.' || prev == '$' || prev == '_' || unicode.IsLetter(prev) || unicode.IsDigit(prev) {
					continue
				}
			}
			prefix := strings.TrimSpace(line[:idx])
			if prefix == "" || strings.HasSuffix(prefix, ".") || strings.HasSuffix(prefix, "=") || strings.HasSuffix(prefix, "(") || strings.HasSuffix(prefix, ",") {
				continue
			}
			candidates = append(candidates, candidate{line: i + 1, col: idx})
		}
	}
	if len(candidates) != 1 {
		return 0, 0, fmt.Errorf("method declaration matches=%d", len(candidates))
	}
	c := candidates[0]
	start := c.line
	for start > 1 && strings.HasPrefix(strings.TrimSpace(lines[start-2]), "@") {
		start--
	}

	// Inspect from the exact method token so an enclosing one-line class brace does
	// not widen the selected method range.
	lineOffset := 0
	for i := 0; i < c.line-1; i++ {
		lineOffset += len(lines[i]) + 1
	}
	methodOffset := lineOffset + c.col
	paren := strings.Index(text[methodOffset:], "(")
	if paren < 0 {
		return 0, 0, fmt.Errorf("method opening paren missing")
	}
	bodyFrom := methodOffset + paren + 1
	brace := strings.Index(text[bodyFrom:], "{")
	semi := strings.Index(text[bodyFrom:], ";")
	if semi >= 0 && (brace < 0 || semi < brace) {
		endOffset := bodyFrom + semi
		return start, 1 + strings.Count(text[:endOffset], "\n"), nil
	}
	if brace < 0 {
		return 0, 0, fmt.Errorf("method body missing")
	}
	open := bodyFrom + brace
	endOffset, ok := matchingJavaBrace180(text, open)
	if !ok {
		return 0, 0, fmt.Errorf("method body unbalanced")
	}
	return start, 1 + strings.Count(text[:endOffset], "\n"), nil
}

func matchingJavaBrace180(text string, open int) (int, bool) {
	depth := 0
	inString := byte(0)
	escaped := false
	lineComment := false
	blockComment := false
	for i := open; i < len(text); i++ {
		c := text[i]
		next := byte(0)
		if i+1 < len(text) {
			next = text[i+1]
		}
		if lineComment {
			if c == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if c == '*' && next == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		if c == '/' && next == '/' {
			lineComment = true
			i++
			continue
		}
		if c == '/' && next == '*' {
			blockComment = true
			i++
			continue
		}
		if c == '"' || c == '\'' {
			inString = c
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func xmlStatementRange180(data []byte, symbol string) (int, int, error) {
	dot := strings.LastIndex(symbol, ".")
	if dot < 0 || dot == len(symbol)-1 {
		return 0, 0, fmt.Errorf("mapper statement symbol required")
	}
	id := symbol[dot+1:]
	dec := xml.NewDecoder(bytes.NewReader(data))
	matches := [][2]int{}
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return 0, 0, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		stmtID := ""
		for _, attr := range start.Attr {
			if attr.Name.Local == "id" {
				stmtID = attr.Value
				break
			}
		}
		if stmtID != id {
			continue
		}
		afterStart := int(dec.InputOffset())
		open := bytes.LastIndex(data[:afterStart], []byte("<"))
		if open < 0 {
			return 0, 0, fmt.Errorf("statement opening tag missing")
		}
		startLine := 1 + bytes.Count(data[:open], []byte("\n"))
		if err := dec.Skip(); err != nil {
			return 0, 0, err
		}
		endOffset := int(dec.InputOffset())
		if endOffset > len(data) {
			endOffset = len(data)
		}
		endLine := 1 + bytes.Count(data[:endOffset], []byte("\n"))
		matches = append(matches, [2]int{startLine, endLine})
	}
	if len(matches) != 1 {
		return 0, 0, fmt.Errorf("mapper statement matches=%d", len(matches))
	}
	return matches[0][0], matches[0][1], nil
}

func validateFindingsWithinSelectedScope180(root string, findings []Finding, allowed []ReadRef) error {
	if len(findings) == 0 {
		return nil
	}
	if len(allowed) == 0 {
		return fmt.Errorf("REVIEW_FINISH_FINDING_OUTSIDE_SCOPE: finding scope unavailable")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, finding := range findings {
		for _, ev := range finding.Evidence {
			ranges, err := evidenceQuoteRanges180(rootAbs, ev)
			if err != nil {
				return err
			}
			inside := 0
			outside := 0
			for _, r := range ranges {
				if rangeAllowed180(r, allowed) {
					inside++
				} else {
					outside++
				}
			}
			if inside == 0 || outside > 0 {
				return fmt.Errorf("REVIEW_FINISH_FINDING_OUTSIDE_SCOPE: %s: %s", finding.ID, ev.Ref.Path)
			}
		}
	}
	return nil
}

func evidenceQuoteRanges180(rootAbs string, ev Evidence) ([]ReadRef, error) {
	data, err := os.ReadFile(filepath.Join(rootAbs, filepath.Clean(filepath.FromSlash(ev.Ref.Path))))
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(data, []byte("\n"))
	if ev.Ref.StartLine < 1 || ev.Ref.EndLine < ev.Ref.StartLine || ev.Ref.EndLine > len(lines) {
		return nil, fmt.Errorf("REVIEW_FINISH_READ_RANGE_INVALID: %s", ev.Ref.Path)
	}
	segment := evidenceVisibleSegment180(lines, ev.Ref.StartLine, ev.Ref.EndLine)
	quote := normalizeEvidenceQuote180(ev.Quote)
	out := []ReadRef{}
	for from := 0; from <= len(segment); {
		i := bytes.Index(segment[from:], quote)
		if i < 0 {
			break
		}
		at := from + i
		start := ev.Ref.StartLine + bytes.Count(segment[:at], []byte("\n"))
		end := start + bytes.Count(quote, []byte("\n"))
		out = append(out, ReadRef{Path: ev.Ref.Path, SHA256: ev.Ref.SHA256, StartLine: start, EndLine: end})
		step := len(quote)
		if step == 0 {
			step = 1
		}
		from = at + step
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("REVIEW_FINISH_EVIDENCE_QUOTE_MISMATCH: %s", ev.Ref.Path)
	}
	return out, nil
}

func normalizeEvidenceQuote180(quote string) []byte {
	return []byte(strings.ReplaceAll(quote, "\r\n", "\n"))
}

func evidenceVisibleSegment180(lines [][]byte, startLine, endLine int) []byte {
	visible := make([][]byte, 0, endLine-startLine+1)
	for _, line := range lines[startLine-1 : endLine] {
		visible = append(visible, bytes.TrimSuffix(line, []byte("\r")))
	}
	return bytes.Join(visible, []byte("\n"))
}

func rangeAllowed180(ref ReadRef, allowed []ReadRef) bool {
	path := filepath.ToSlash(filepath.Clean(ref.Path))
	for _, candidate := range allowed {
		if filepath.ToSlash(filepath.Clean(candidate.Path)) == path && candidate.SHA256 == ref.SHA256 && ref.StartLine >= candidate.StartLine && ref.EndLine <= candidate.EndLine {
			return true
		}
	}
	return false
}
