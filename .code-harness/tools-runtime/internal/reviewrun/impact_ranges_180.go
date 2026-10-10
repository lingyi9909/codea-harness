package reviewrun

import (
    "bytes"
    "context"
    "encoding/xml"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strconv"
    "codea-harness-tools/internal/changeset"
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

// buildImpactRanges180 uses one snapshot and one bounded Git diff, not one
// snapshot+subprocess per changed Java/XML file. This is critical for the
// hundreds-of-Controller-methods enterprise acceptance budget.
func buildImpactRanges180(ctx context.Context, root string, paths []string) (*impactRanges180, error) {
    idx := &impactRanges180{changed: map[string][]ReadRef{}, mapped: map[string][]ReadRef{}, gaps: []string{}}
    snapshot, err := reviewChangesSnapshot180(root)
    if err != nil {return nil,fmt.Errorf("CHANGE_IMPACT_DIFF_FAILED: %w",err)}
    untracked:=map[string]bool{}
    for _,file:=range snapshot.Files {
        for _,source:=range file.Sources {
            if source==changeset.SourceUntracked {untracked[filepath.ToSlash(file.Path)]=true}
        }
    }
    args:=[]string{"-c","core.quotePath=false","diff","--unified=0","--no-ext-diff","--no-color","--no-renames",snapshot.MergeBase}
    if !snapshot.IncludeWorkingTree {args=append(args,snapshot.HeadCommit)}
    args=append(args,"--")
    cmd:=exec.CommandContext(ctx,"git",args...)
    cmd.Dir=root
    raw,err:=cmd.Output()
    if err!=nil {return nil,fmt.Errorf("CHANGE_IMPACT_DIFF_FAILED: %w",err)}
    // The +++ destination declares the exact current path. Git C-quotes
    // unusual names; ambiguous or malformed headers fail closed below.
    current:=""
    for _,line:=range strings.Split(string(raw),"\n") {
        if strings.HasPrefix(line,"diff --git ") {current="";continue}
        if strings.HasPrefix(line,"+++ ") {
            current=""
            header:=strings.TrimSpace(strings.TrimPrefix(line,"+++ "))
            if header=="/dev/null" {continue}
            if strings.HasPrefix(header,"\"") {
                decoded,e:=strconv.Unquote(header)
                if e!=nil {return nil,fmt.Errorf("CHANGE_IMPACT_DIFF_FAILED: invalid Git path quoting")}
                header=decoded
            }
            if strings.HasPrefix(header,"b/") {
                current=filepath.ToSlash(strings.TrimPrefix(header,"b/"))
            }
            continue
        }
        if current=="" {continue}
        m:=diffHunk180.FindStringSubmatch(line)
        if len(m)==0 {continue}
        first,e:=strconv.Atoi(m[1])
        if e!=nil {return nil,e}
        count:=1
        if m[2]!="" {count,e=strconv.Atoi(m[2]);if e!=nil{return nil,e}}
        if count>0 {
            idx.changed[current]=append(idx.changed[current],ReadRef{Path:current,StartLine:first,EndLine:first+count-1})
        }
    }
    for _,path:=range paths {
        path=filepath.ToSlash(path)
        if untracked[path] {
            idx.changed[path],err=fullFileReadRange180(root,path)
            if err!=nil {return nil,err}
        }
        if len(idx.changed[path])==0 {
            idx.gaps=append(idx.gaps,"CHANGE_IMPACT_UNRESOLVED: no current-line diff range for "+path)
        }
    }
    return idx,nil
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
    statementRanges := []ReadRef{}
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
        span:=ReadRef{Path:filepath.ToSlash(rel),StartLine:1+bytes.Count(data[:opening],[]byte("\n")),EndLine:1+bytes.Count(data[:end],[]byte("\n"))}
        statementRanges=append(statementRanges,span)
        if namespace+"."+id != identity {continue}
        found=append(found,span)
    }
    if len(found)!=1 { return ReadRef{},fmt.Errorf("Mapper XML statement not unique: %s in %s count=%d",identity,rel,len(found)) }
    for _,other:=range statementRanges {
        if other.StartLine==found[0].StartLine && other.EndLine==found[0].EndLine {continue}
        if other.StartLine<=found[0].EndLine && other.EndLine>=found[0].StartLine {
            return ReadRef{},fmt.Errorf("Mapper XML statements share a source line: %s in %s",identity,rel)
        }
    }
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
