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
	"unicode/utf8"
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

type sourceSpan180 struct {
	Start int
	End   int
}

type javaToken180 struct {
	Text  string
	Start int
	End   int
	Ident bool
}

func javaMethodRange180(data []byte, symbol string) (int, int, error) {
	normalized := normalizeSource180(data)
	span, err := javaMethodSpan180(normalized, symbol)
	if err != nil {
		return 0, 0, err
	}
	return spanLineRange180(normalized, span)
}

func javaMethodSpan180(data []byte, symbol string) (sourceSpan180, error) {
	dot := strings.LastIndex(symbol, ".")
	if dot < 0 || dot == len(symbol)-1 {
		return sourceSpan180{}, fmt.Errorf("method symbol required")
	}
	method := symbol[dot+1:]
	text := string(data)
	tokens := lexJava180(text)
	candidates := []sourceSpan180{}
	for i := 0; i+1 < len(tokens); i++ {
		if !tokens[i].Ident || tokens[i].Text != method || tokens[i+1].Text != "(" {
			continue
		}
		if !javaDeclarationPrefix180(tokens, i) {
			continue
		}
		closeParen, ok := matchingJavaTokenPair180(tokens, i+1, "(", ")")
		if !ok {
			continue
		}
		terminator, ok := javaMethodTerminator180(tokens, closeParen+1)
		if !ok {
			continue
		}
		startIndex := javaDeclarationStart180(tokens, i)
		end := tokens[terminator].End
		if tokens[terminator].Text == "{" {
			closeBrace, matched := matchingJavaTokenPair180(tokens, terminator, "{", "}")
			if !matched {
				continue
			}
			end = tokens[closeBrace].End
		}
		candidates = append(candidates, sourceSpan180{Start: tokens[startIndex].Start, End: end})
	}
	if len(candidates) != 1 {
		return sourceSpan180{}, fmt.Errorf("method declaration matches=%d", len(candidates))
	}
	return candidates[0], nil
}

func javaDeclarationPrefix180(tokens []javaToken180, methodIndex int) bool {
	if methodIndex == 0 {
		return false
	}
	prev := tokens[methodIndex-1]
	if prev.Text == "." || prev.Text == "::" {
		return false
	}
	if prev.Ident {
		switch prev.Text {
		case "return", "throw", "new", "case", "yield", "if", "else", "for", "while", "switch", "catch", "assert", "this", "super":
			return false
		}
		return true
	}
	return prev.Text == "]" || prev.Text == ">"
}

func javaDeclarationStart180(tokens []javaToken180, methodIndex int) int {
	start := 0
	for i := methodIndex - 1; i >= 0; i-- {
		switch tokens[i].Text {
		case ";", "{", "}":
			return i + 1
		}
	}
	return start
}

func javaMethodTerminator180(tokens []javaToken180, from int) (int, bool) {
	if from >= len(tokens) {
		return 0, false
	}
	for i := from; i < len(tokens); i++ {
		switch tokens[i].Text {
		case "{", ";":
			return i, true
		case "=", "->", "(", ")", "}":
			return 0, false
		}
	}
	return 0, false
}

func matchingJavaTokenPair180(tokens []javaToken180, openIndex int, open, close string) (int, bool) {
	depth := 0
	for i := openIndex; i < len(tokens); i++ {
		switch tokens[i].Text {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func lexJava180(text string) []javaToken180 {
	tokens := []javaToken180{}
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "//") {
			if j := strings.IndexByte(text[i+2:], '\n'); j >= 0 {
				i += 2 + j + 1
			} else {
				break
			}
			continue
		}
		if strings.HasPrefix(text[i:], "/*") {
			if j := strings.Index(text[i+2:], "*/"); j >= 0 {
				i += 2 + j + 2
			} else {
				break
			}
			continue
		}
		if strings.HasPrefix(text[i:], "\"\"\"") {
			if j := strings.Index(text[i+3:], "\"\"\""); j >= 0 {
				i += 3 + j + 3
			} else {
				break
			}
			continue
		}
		if text[i] == '"' || text[i] == '\'' {
			i = skipJavaQuoted180(text, i, text[i])
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		if isJavaIdentStart180(r) {
			start := i
			i += size
			for i < len(text) {
				r, size = utf8.DecodeRuneInString(text[i:])
				if !isJavaIdentPart180(r) {
					break
				}
				i += size
			}
			tokens = append(tokens, javaToken180{Text: text[start:i], Start: start, End: i, Ident: true})
			continue
		}
		if i+1 < len(text) {
			pair := text[i : i+2]
			if pair == "::" || pair == "->" {
				tokens = append(tokens, javaToken180{Text: pair, Start: i, End: i + 2})
				i += 2
				continue
			}
		}
		tokens = append(tokens, javaToken180{Text: text[i : i+size], Start: i, End: i + size})
		i += size
	}
	return tokens
}

func skipJavaQuoted180(text string, start int, quote byte) int {
	for i := start + 1; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if text[i] == quote {
			return i + 1
		}
	}
	return len(text)
}

func isJavaIdentStart180(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r)
}

func isJavaIdentPart180(r rune) bool {
	return isJavaIdentStart180(r) || unicode.IsDigit(r)
}

func normalizeSource180(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func spanLineRange180(data []byte, span sourceSpan180) (int, int, error) {
	if span.Start < 0 || span.End <= span.Start || span.End > len(data) {
		return 0, 0, fmt.Errorf("source span invalid")
	}
	start := 1 + bytes.Count(data[:span.Start], []byte("\n"))
	endOffset := span.End - 1
	end := 1 + bytes.Count(data[:endOffset], []byte("\n"))
	return start, end, nil
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
	normalized := normalizeSource180(data)
	span, err := xmlStatementSpan180(normalized, symbol)
	if err != nil {
		return 0, 0, err
	}
	return spanLineRange180(normalized, span)
}

func xmlStatementSpan180(data []byte, symbol string) (sourceSpan180, error) {
	dot := strings.LastIndex(symbol, ".")
	if dot < 0 || dot == len(symbol)-1 {
		return sourceSpan180{}, fmt.Errorf("mapper statement symbol required")
	}
	id := symbol[dot+1:]
	dec := xml.NewDecoder(bytes.NewReader(data))
	matches := []sourceSpan180{}
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return sourceSpan180{}, err
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
			return sourceSpan180{}, fmt.Errorf("statement opening tag missing")
		}
		if err := dec.Skip(); err != nil {
			return sourceSpan180{}, err
		}
		endOffset := int(dec.InputOffset())
		if endOffset > len(data) {
			endOffset = len(data)
		}
		matches = append(matches, sourceSpan180{Start: open, End: endOffset})
	}
	if len(matches) != 1 {
		return sourceSpan180{}, fmt.Errorf("mapper statement matches=%d", len(matches))
	}
	return matches[0], nil
}

type findingSpan180 struct {
	Path   string
	SHA256 string
	Start  int
	End    int
}

func validateFindingsWithinSelectedScope180(root string, findings []Finding, chains []Chain) error {
	if len(findings) == 0 {
		return nil
	}
	allowed, err := findingSpansForChains180(root, chains)
	if err != nil {
		return err
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
			spans, err := evidenceQuoteSpans180(rootAbs, ev)
			if err != nil {
				return err
			}
			inside := 0
			outside := 0
			for _, span := range spans {
				if findingSpanAllowed180(span, allowed) {
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

func findingSpansForChains180(root string, chains []Chain) ([]findingSpan180, error) {
	out := []findingSpan180{}
	seen := map[string]bool{}
	for _, chain := range chains {
		for _, node := range chain.Nodes {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(node.Path)))
			if err != nil {
				return nil, fmt.Errorf("REVIEW_FINDING_SCOPE_READ_FAILED: %s: %w", node.Path, err)
			}
			normalized := normalizeSource180(raw)
			var span sourceSpan180
			if strings.EqualFold(node.Role, "SQL") {
				span, err = xmlStatementSpan180(normalized, node.Symbol)
			} else {
				span, err = javaMethodSpan180(normalized, node.Symbol)
			}
			if err != nil {
				return nil, fmt.Errorf("REVIEW_FINDING_SCOPE_UNRESOLVED: %s %s: %w", node.Path, node.Symbol, err)
			}
			item := findingSpan180{
				Path: filepath.ToSlash(filepath.Clean(node.Path)), SHA256: bytesSHA256(raw),
				Start: span.Start, End: span.End,
			}
			key := fmt.Sprintf("%s|%s|%d|%d", item.Path, item.SHA256, item.Start, item.End)
			if !seen[key] {
				seen[key] = true
				out = append(out, item)
			}
		}
	}
	return out, nil
}

func evidenceQuoteSpans180(rootAbs string, ev Evidence) ([]findingSpan180, error) {
	raw, err := os.ReadFile(filepath.Join(rootAbs, filepath.Clean(filepath.FromSlash(ev.Ref.Path))))
	if err != nil {
		return nil, err
	}
	data := normalizeSource180(raw)
	lines := bytes.Split(data, []byte("\n"))
	if ev.Ref.StartLine < 1 || ev.Ref.EndLine < ev.Ref.StartLine || ev.Ref.EndLine > len(lines) {
		return nil, fmt.Errorf("REVIEW_FINISH_READ_RANGE_INVALID: %s", ev.Ref.Path)
	}
	base := 0
	for i := 0; i < ev.Ref.StartLine-1; i++ {
		base += len(lines[i]) + 1
	}
	segment := bytes.Join(lines[ev.Ref.StartLine-1:ev.Ref.EndLine], []byte("\n"))
	quote := normalizeEvidenceQuote180(ev.Quote)
	out := []findingSpan180{}
	for from := 0; from <= len(segment); {
		i := bytes.Index(segment[from:], quote)
		if i < 0 {
			break
		}
		at := from + i
		start := base + at
		end := start + len(quote)
		start, end = trimEvidenceBoundaryWhitespace180(data, start, end)
		out = append(out, findingSpan180{
			Path: filepath.ToSlash(filepath.Clean(ev.Ref.Path)), SHA256: ev.Ref.SHA256,
			Start: start, End: end,
		})
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

func trimEvidenceBoundaryWhitespace180(data []byte, start, end int) (int, int) {
	for start < end {
		switch data[start] {
		case ' ', '\t', '\n', '\r':
			start++
		default:
			goto trimEnd
		}
	}
trimEnd:
	for end > start {
		switch data[end-1] {
		case ' ', '\t', '\n', '\r':
			end--
		default:
			return start, end
		}
	}
	return start, end
}

func findingSpanAllowed180(ref findingSpan180, allowed []findingSpan180) bool {
	for _, candidate := range allowed {
		if candidate.Path == ref.Path && candidate.SHA256 == ref.SHA256 &&
			ref.Start >= candidate.Start && ref.End <= candidate.End {
			return true
		}
	}
	return false
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
