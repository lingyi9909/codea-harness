package reviewrun

import (
    "bytes"
    "context"
    "encoding/xml"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "codea-harness-tools/internal/nav"
)

// impactRanges180 is a per-prepare immutable Git diff index. Every candidate
// is checked against exact method/statement intervals, not its entire file.
// A missing or ambiguous interval must never prove a CHANGES auto-selection.
type impactRanges180 struct {
    changed map[string][]ReadRef
    mapped map[string][]ReadRef
    gaps []string
}

func buildImpactRanges180(ctx context.Context, root string, paths []string) (*impactRanges180, error) {
    idx := &impactRanges180{changed: map[string][]ReadRef{}, mapped: map[string][]ReadRef{}, gaps: []string{}}
    for _, path := range paths {
        ranges, err := changedLineRangesForPath180(ctx, root, path)
        if err != nil {
            return nil, fmt.Errorf("CHANGE_IMPACT_DIFF_FAILED: %s: %w", path, err)
        }
        idx.changed[path] = ranges
        if len(ranges) == 0 {
            idx.gaps = append(idx.gaps, "CHANGE_IMPACT_UNRESOLVED: no current-line diff range for "+path)
        }
    }
    return idx, nil
}

func (idx *impactRanges180) intersects(path string, start, end int) bool {
    if idx == nil || start < 1 || end < start { return false }
    path = filepath.ToSlash(path)
    if _, changed := idx.changed[path]; !changed { return false }
    ref := ReadRef{Path:path, StartLine:start, EndLine:end}
    idx.mapped[path] = append(idx.mapped[path],ref)
    return overlapsChangedRange180(ref,idx.changed[path])
}

func (idx *impactRanges180) method(spans []nav.MethodSpan180, expectedPath, symbol string) bool {
    if idx == nil { return false }
    // Overloads/duplicate declarations cannot be attributed from a name alone.
    matches := []nav.MethodSpan180{}
    for _, s := range spans {
        if filepath.ToSlash(s.Path) == filepath.ToSlash(expectedPath) { matches = append(matches,s) }
    }
    if len(matches) != 1 {
        if _, changed := idx.changed[filepath.ToSlash(expectedPath)]; changed {
            idx.gaps=append(idx.gaps,"CHANGE_IMPACT_UNRESOLVED: method declaration "+symbol+" in "+expectedPath)
        }
        return false
    }
    return idx.intersects(matches[0].Path,matches[0].StartLine,matches[0].EndLine)
}

func (idx *impactRanges180) statement(root, path, namespaceAndID string) bool {
    if idx == nil { return false }
    if _, changed := idx.changed[filepath.ToSlash(path)]; !changed { return false }
    span, err := mapperStatementRange180(root,path,namespaceAndID)
    if err != nil {
        idx.gaps=append(idx.gaps,"CHANGE_IMPACT_UNRESOLVED: "+err.Error())
        return false
    }
    return idx.intersects(path,span.StartLine,span.EndLine)
}

// mapperStatementRange180 uses the XML parser, not a free-form text regex;
// multi-line MyBatis statements stay bounded, and namespace/id collisions
// or malformed XML cannot silently make an unrelated SQL statement affected.
func mapperStatementRange180(root, rel, identity string) (ReadRef, error) {
    data, err := os.ReadFile(filepath.Join(root,filepath.FromSlash(rel)))
    if err != nil { return ReadRef{},err }
    dec := xml.NewDecoder(bytes.NewReader(data))
    namespace := ""
    found := []ReadRef{}
    for {
        token, err := dec.Token()
        if err == io.EOF { break }
        if err != nil { return ReadRef{},fmt.Errorf("invalid Mapper XML %s: %w",rel,err) }
        start, ok := token.(xml.StartElement)
        if !ok { continue }
        if start.Name.Local=="mapper" {
            for _, a := range start.Attr { if a.Name.Local=="namespace" {namespace=strings.TrimSpace(a.Value)} }
            continue
        }
        id:=""
        for _,a:=range start.Attr { if a.Name.Local=="id" {id=a.Value;break} }
        if id=="" { continue }
        endOfOpeningTag:=int(dec.InputOffset())
        opening:=bytes.LastIndexByte(data[:endOfOpeningTag],'<')
        if opening<0 { return ReadRef{},fmt.Errorf("Mapper XML opening tag missing: %s",rel) }
        if err:=dec.Skip();err!=nil {return ReadRef{},err}
        end:=int(dec.InputOffset())
        if namespace+"."+id != identity {continue}
        found=append(found,ReadRef{Path:filepath.ToSlash(rel),StartLine:1+bytes.Count(data[:opening],[]byte("\n")),EndLine:1+bytes.Count(data[:end],[]byte("\n"))})
    }
    if len(found)!=1 { return ReadRef{},fmt.Errorf("Mapper XML statement not unique: %s in %s count=%d",identity,rel,len(found)) }
    return found[0],nil
}

func (idx *impactRanges180) uncoveredGaps() []string {
    if idx==nil {return nil}
    gaps:=append([]string{},idx.gaps...)
    for path, changes := range idx.changed {
        for _, changed := range changes {
            // Every individual diff hunk must map to some proven method or
            // statement. A touched sibling method cannot be hidden by the
            // existence of a changed method elsewhere in the same file.
            if !overlapsAnyImpact180(changed,idx.mapped[path]) {
                gaps=append(gaps,fmt.Sprintf("CHANGE_IMPACT_UNRESOLVED: %s:%d-%d lacks a proven method/statement",path,changed.StartLine,changed.EndLine))
            }
        }
    }
    return uniqueStrings180(gaps)
}

func overlapsAnyImpact180(needle ReadRef, haystack []ReadRef) bool {
    return overlapsChangedRange180(needle,haystack)
}
