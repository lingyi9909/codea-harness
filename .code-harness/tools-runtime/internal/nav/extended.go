package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

var ErrAmbiguousSymbol = errors.New("AMBIGUOUS_SYMBOL")
var ErrSymbolNotFound = errors.New("SYMBOL_NOT_FOUND")
var annotationNameRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
var identRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

type SymbolInfo struct { Symbol string `json:"symbol"`; Kind string `json:"kind"`; DeclaringType string `json:"declaringType"`; Signature string `json:"signature"`; ReturnType string `json:"returnType,omitempty"`; Annotations []string `json:"annotations"`; Path string `json:"path"`; LineStart int `json:"lineStart"`; LineEnd int `json:"lineEnd"` }
type AnnotationMatch struct { Symbol string `json:"symbol"`; Kind string `json:"kind"`; Annotation string `json:"annotation"`; Path string `json:"path"`; LineStart int `json:"lineStart"`; LineEnd int `json:"lineEnd"` }
type AnnotationResult struct { Annotation string `json:"annotation"`; Scope string `json:"scope"`; Matches []AnnotationMatch `json:"matches"` }
type CallerMatch struct { CallerSymbol string `json:"callerSymbol"`; Path string `json:"path"`; Line int `json:"line"`; Receiver string `json:"receiver,omitempty"`; ReceiverType string `json:"receiverType,omitempty"`; Resolution string `json:"resolution"` }
type CallerResult struct { Symbol string `json:"symbol"`; Scope string `json:"scope"`; Callers []CallerMatch `json:"callers"`; Candidates []CallerMatch `json:"candidates,omitempty"` }
type rawMatch struct { Path, Text string; StartLine, StartColumn, EndLine, EndColumn int }
type sgExtendedLine struct { File string `json:"file"`; Text string `json:"text"`; Range struct { Start struct { Line int `json:"line"`; Column int `json:"column"` } `json:"start"`; End struct { Line int `json:"line"`; Column int `json:"column"` } `json:"end"` } `json:"range"` }

func (n Navigator) runRaw(ctx context.Context, scope string, patterns ...string) ([]rawMatch,error) {
	if err:=n.validate("X",scope); err!=nil { return nil,err }
	r:=n.Runner; if r==nil { r=ExecRunner{} }
	seen:=map[string]bool{}; var out []rawMatch
	for _,p:=range patterns {
		args:=[]string{"--lang","java","--json=stream","--pattern",p,strings.ReplaceAll(scope,"\\","/")}
		b,err:=r.Run(ctx,n.AstGrepPath,args...)
		if err!=nil { var ee *exec.ExitError; if !errors.As(err,&ee){return nil,err}; if len(b)==0{continue} }
		s:=bufio.NewScanner(bytes.NewReader(b))
		for s.Scan(){ var x sgExtendedLine; if json.Unmarshal(s.Bytes(),&x)!=nil{continue}; m:=rawMatch{Path:strings.ReplaceAll(x.File,"\\","/"),Text:x.Text,StartLine:x.Range.Start.Line+1,StartColumn:x.Range.Start.Column+1,EndLine:x.Range.End.Line+1,EndColumn:x.Range.End.Column+1}; if m.EndLine<m.StartLine{m.EndLine=m.StartLine}; k:=fmt.Sprintf("%s:%d:%d:%d:%d:%s",m.Path,m.StartLine,m.StartColumn,m.EndLine,m.EndColumn,m.Text); if !seen[k]{seen[k]=true; out=append(out,m)} }
		if err:=s.Err(); err!=nil{return nil,err}
	}
	return out,nil
}

func typePatterns(name string) []string { return []string{"class "+name+" { $$$BODY }","public class "+name+" { $$$BODY }","final class "+name+" { $$$BODY }","public final class "+name+" { $$$BODY }","abstract class "+name+" { $$$BODY }","public abstract class "+name+" { $$$BODY }","interface "+name+" { $$$BODY }","public interface "+name+" { $$$BODY }","enum "+name+" { $$$BODY }","public enum "+name+" { $$$BODY }"} }
func allTypePatterns() []string { return []string{"class $C { $$$BODY }","public class $C { $$$BODY }","final class $C { $$$BODY }","public final class $C { $$$BODY }","abstract class $C { $$$BODY }","public abstract class $C { $$$BODY }","interface $C { $$$BODY }","public interface $C { $$$BODY }","enum $C { $$$BODY }","public enum $C { $$$BODY }"} }
func methodPatterns(name string) []string { bases:=[]string{"$RET "+name+"($$$ARGS) { $$$BODY }","$RET "+name+"($$$ARGS);"}; mods:=[]string{"public ","protected ","private ","static ","public static ","protected static ","private static ","final ","public final ","abstract ","public abstract ","default "}; out:=append([]string{},bases...); for _,m:=range mods{out=append(out,m+bases[0],m+bases[1])}; return out }
func allMethodPatterns() []string { bases:=[]string{"$RET $M($$$ARGS) { $$$BODY }","$RET $M($$$ARGS);"}; mods:=[]string{"public ","protected ","private ","static ","public static ","protected static ","private static ","final ","public final ","abstract ","public abstract ","default "}; out:=append([]string{},bases...); for _,m:=range mods{out=append(out,m+bases[0],m+bases[1])}; return out }
func fieldPatterns(name string) []string { bases:=[]string{"$T "+name+";","$T "+name+" = $INIT;"}; mods:=[]string{"public ","protected ","private ","static ","public static ","private static ","final ","private final ","public final ","private static final ","public static final "}; out:=append([]string{},bases...); for _,m:=range mods{out=append(out,m+bases[0],m+bases[1])}; return out }
func annotationPatterns(name string) []string { prefixes:=[]string{"@"+name+" ","@"+name+"($$$ANNARGS) "}; decls:=append(allTypePatterns(),allMethodPatterns()...); var out []string; for _,pre:=range prefixes{for _,d:=range decls{out=append(out,pre+d)}}; return out }

func (n Navigator) GetSymbolInfo(ctx context.Context,symbol,scope string)(SymbolInfo,error){
	if err:=n.validate(symbol,scope);err!=nil{return SymbolInfo{},err}; owner,member:=splitSymbol(symbol); types,err:=n.runRaw(ctx,scope,typePatterns(owner)...); if err!=nil{return SymbolInfo{},err}; types=filterTypeName(types,owner)
	if member==""{if len(types)==0{return SymbolInfo{},ErrSymbolNotFound}; if len(types)>1{return SymbolInfo{},ErrAmbiguousSymbol}; return makeTypeInfo(symbol,types[0],owner),nil}
	methods,err:=n.runRaw(ctx,scope,methodPatterns(member)...); if err!=nil{return SymbolInfo{},err}; fields,err:=n.runRaw(ctx,scope,fieldPatterns(member)...); if err!=nil{return SymbolInfo{},err}; var infos []SymbolInfo
	for _,typ:=range types{for _,m:=range methods{if contains(typ,m)&&methodName(m.Text)==member{infos=append(infos,makeMethodInfo(symbol,owner,member,m))}}; for _,f:=range fields{if contains(typ,f)&&fieldName(f.Text)==member{infos=append(infos,makeFieldInfo(symbol,owner,member,f))}}}
	infos=dedupeInfos(infos); if len(infos)==0{return SymbolInfo{},ErrSymbolNotFound}; if len(infos)>1{return SymbolInfo{},ErrAmbiguousSymbol}; return infos[0],nil
}

func (n Navigator) FindByAnnotation(ctx context.Context,annotation,scope string)(AnnotationResult,error){
	if !annotationNameRE.MatchString(annotation){return AnnotationResult{},ErrInvalidSymbol}; if err:=n.validate("X",scope);err!=nil{return AnnotationResult{},err}; matches,err:=n.runRaw(ctx,scope,annotationPatterns(annotation)...); if err!=nil{return AnnotationResult{},err}; types,err:=n.runRaw(ctx,scope,allTypePatterns()...); if err!=nil{return AnnotationResult{},err}
	var out []AnnotationMatch; seen:=map[string]bool{}
	for _,m:=range matches{ann:=findAnnotation(m.Text,annotation); if ann==""{continue}; if kind,name:=typeKindAndName(m.Text); kind!=""{k:=m.Path+fmt.Sprint(m.StartLine)+name; if !seen[k]{seen[k]=true; out=append(out,AnnotationMatch{Symbol:name,Kind:kind,Annotation:ann,Path:m.Path,LineStart:m.StartLine,LineEnd:m.EndLine})}; continue}; name:=methodName(m.Text); if name==""{continue}; owner:=enclosingTypeName(types,m); if owner==""{continue}; k:=m.Path+fmt.Sprint(m.StartLine)+owner+name; if !seen[k]{seen[k]=true; out=append(out,AnnotationMatch{Symbol:owner+"."+name,Kind:"METHOD",Annotation:ann,Path:m.Path,LineStart:m.StartLine,LineEnd:m.EndLine})}}
	sort.Slice(out,func(i,j int)bool{if out[i].Path==out[j].Path{return out[i].LineStart<out[j].LineStart};return out[i].Path<out[j].Path}); return AnnotationResult{Annotation:annotation,Scope:scope,Matches:out},nil
}

func (n Navigator) FindCallers(ctx context.Context,symbol,scope string)(CallerResult,error){
	if err:=n.validate(symbol,scope);err!=nil{return CallerResult{},err}; owner,member:=splitSymbol(symbol); if member==""{return CallerResult{},ErrInvalidSymbol}
	calls,err:=n.runRaw(ctx,scope,"$OBJ."+member+"($$$ARGS)",member+"($$$ARGS)"); if err!=nil{return CallerResult{},err}; methods,err:=n.runRaw(ctx,scope,allMethodPatterns()...); if err!=nil{return CallerResult{},err}; types,err:=n.runRaw(ctx,scope,allTypePatterns()...); if err!=nil{return CallerResult{},err}
	var confirmed,candidates []CallerMatch; seen:=map[string]bool{}
	for _,call:=range calls{method,ok:=smallestContaining(methods,call); if !ok{continue}; mn:=methodName(method.Text); if mn==""{continue}; caller:=mn; if t:=enclosingTypeName(types,method);t!=""{caller=t+"."+mn}; recv:=receiverName(call.Text,member); cm:=CallerMatch{CallerSymbol:caller,Path:call.Path,Line:call.StartLine,Receiver:recv}
		if recv==""{cm.Resolution="CANDIDATE"; candidates=appendUniqueCaller(candidates,seen,cm); continue}
		declName:=recv; if i:=strings.LastIndex(declName,".");i>=0{declName=declName[i+1:]}; if !identRE.MatchString(declName){cm.Resolution="CANDIDATE"; candidates=appendUniqueCaller(candidates,seen,cm); continue}
		decls,err:=n.runRaw(ctx,scope,fieldPatterns(declName)...); if err!=nil{return CallerResult{},err}; typeSet:=map[string]bool{}; for _,d:=range decls{if d.Path==call.Path{if typ:=simpleType(fieldType(d.Text,declName));typ!=""{typeSet[typ]=true}}}
		if len(typeSet)==1{for typ:=range typeSet{cm.ReceiverType=typ; if typ==simpleType(owner){cm.Resolution="CONFIRMED"; confirmed=appendUniqueCaller(confirmed,seen,cm)} /* mismatched receiver type is excluded */ }} else {cm.Resolution="CANDIDATE"; candidates=appendUniqueCaller(candidates,seen,cm)}
	}
	sortCallers(confirmed); sortCallers(candidates); return CallerResult{Symbol:symbol,Scope:scope,Callers:confirmed,Candidates:candidates},nil
}

func receiverName(text,member string) string { s:=strings.TrimSpace(text); needle:="."+member+"("; i:=strings.Index(s,needle); if i<0{return ""}; left:=strings.TrimSpace(s[:i]); parts:=strings.Fields(left); if len(parts)>0{return parts[len(parts)-1]}; return left }
func appendUniqueCaller(dst []CallerMatch,seen map[string]bool,c CallerMatch)[]CallerMatch{ k:=fmt.Sprintf("%s:%d:%s:%s",c.Path,c.Line,c.CallerSymbol,c.Resolution); if seen[k]{return dst}; seen[k]=true; return append(dst,c) }
func sortCallers(v []CallerMatch){sort.Slice(v,func(i,j int)bool{if v[i].Path==v[j].Path{return v[i].Line<v[j].Line};return v[i].Path<v[j].Path})}
func simpleType(s string) string { s=strings.TrimSpace(s); if i:=strings.Index(s,"<");i>=0{s=s[:i]}; s=strings.TrimSuffix(s,"[]"); if i:=strings.LastIndex(s,".");i>=0{s=s[i+1:]}; return s }

func filterTypeName(ms []rawMatch,name string)[]rawMatch{var out []rawMatch;for _,m:=range ms{_,n:=typeKindAndName(m.Text);if n==name{out=append(out,m)}};return dedupeRaw(out)}
func dedupeRaw(ms []rawMatch)[]rawMatch{seen:=map[string]bool{};out:=ms[:0];for _,m:=range ms{k:=fmt.Sprintf("%s:%d:%d",m.Path,m.StartLine,m.EndLine);if !seen[k]{seen[k]=true;out=append(out,m)}};return out}
func dedupeInfos(in []SymbolInfo)[]SymbolInfo{seen:=map[string]bool{};var out []SymbolInfo;for _,v:=range in{k:=fmt.Sprintf("%s:%d:%d:%s",v.Path,v.LineStart,v.LineEnd,v.Kind);if !seen[k]{seen[k]=true;out=append(out,v)}};return out}
func contains(o,i rawMatch)bool{return o.Path==i.Path&&o.StartLine<=i.StartLine&&o.EndLine>=i.EndLine}
func smallestContaining(cs []rawMatch,t rawMatch)(rawMatch,bool){var b rawMatch;found:=false;for _,c:=range cs{if !contains(c,t){continue};if !found||(c.EndLine-c.StartLine)<(b.EndLine-b.StartLine){b=c;found=true}};return b,found}
func enclosingTypeName(types []rawMatch,m rawMatch)string{t,ok:=smallestContaining(types,m);if !ok{return ""};_,n:=typeKindAndName(t.Text);return n}
var typeDeclRE=regexp.MustCompile(`\b(class|interface|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
func typeKindAndName(text string)(string,string){m:=typeDeclRE.FindStringSubmatch(text);if len(m)!=3{return "",""};return strings.ToUpper(m[1]),m[2]}

func parseLeadingAnnotations(text string)([]string,string){s:=strings.TrimSpace(strings.ReplaceAll(text,"\r\n","\n"));var anns []string; for strings.HasPrefix(s,"@"){end:=annotationEnd(s); if end<=0{break}; anns=append(anns,normalizeSpace(s[:end])); s=strings.TrimSpace(s[end:])}; return anns,s}
func annotationEnd(s string)int{ i:=1; for i<len(s)&&(isIdentByte(s[i])||s[i]=='.'){i++}; for i<len(s)&&(s[i]==' '||s[i]=='\t'){i++}; if i>=len(s)||s[i]!='(' { for i<len(s)&&s[i]!='\n'{i++}; return i }; close:=balancedClose(s,i); if close<0{return -1}; return close+1 }
func balancedClose(s string,open int)int{depth:=0;quote:=byte(0);esc:=false;for i:=open;i<len(s);i++{c:=s[i];if quote!=0{if esc{esc=false;continue};if c=='\\'{esc=true;continue};if c==quote{quote=0};continue};if c=='"'||c=='\''{quote=c;continue};if c=='('{depth++};if c==')'{depth--;if depth==0{return i}}};return -1}
func isIdentByte(b byte)bool{return b=='_'||b=='$'||b=='.'||b>='0'&&b<='9'||b>='A'&&b<='Z'||b>='a'&&b<='z'}
func normalizeSpace(s string)string{return strings.Join(strings.Fields(s)," ")}
func annotations(text string)[]string{a,_:=parseLeadingAnnotations(text);return a}
func findAnnotation(text,name string)string{prefix:="@"+name;for _,a:=range annotations(text){if a==prefix||strings.HasPrefix(a,prefix+"("){return a}};return ""}
func declarationText(text string)string{_,rest:=parseLeadingAnnotations(text);return strings.TrimSpace(rest)}
func declarationLine(text string)string{rest:=declarationText(text);if rest==""{return ""};return normalizeSpace(rest)}
func methodName(text string)string{line:=declarationLine(text);open:=strings.Index(line,"(");if open<0{return ""};before:=strings.TrimSpace(line[:open]);parts:=strings.Fields(before);if len(parts)==0{return ""};return parts[len(parts)-1]}
func methodParts(text,name string)(sig,ret string){line:=declarationLine(text);needle:=name+"(";i:=strings.Index(line,needle);if i<0{return "",""};open:=i+len(name);close:=balancedClose(line,open);if close<0{return "",""};params:=strings.TrimSpace(line[open+1:close]);sig=name+"("+normalizeSpace(params)+")";before:=strings.TrimSpace(line[:i]);parts:=strings.Fields(before);if len(parts)>0{ret=parts[len(parts)-1]};return sig,ret}
func fieldName(text string)string{line:=declarationLine(text);line=strings.TrimSuffix(strings.TrimSpace(line),";");if i:=strings.Index(line,"=");i>=0{line=strings.TrimSpace(line[:i])};parts:=strings.Fields(line);if len(parts)==0{return ""};return parts[len(parts)-1]}
func fieldType(text,name string)string{line:=declarationLine(text);i:=strings.Index(line,name);if i<0{return ""};before:=strings.TrimSpace(line[:i]);parts:=strings.Fields(before);if len(parts)==0{return ""};return parts[len(parts)-1]}
func makeTypeInfo(symbol string,m rawMatch,name string)SymbolInfo{kind,_:=typeKindAndName(m.Text);return SymbolInfo{Symbol:symbol,Kind:kind,DeclaringType:name,Signature:typeSignature(m.Text,name),Annotations:annotations(m.Text),Path:m.Path,LineStart:m.StartLine,LineEnd:m.EndLine}}
func typeSignature(text,name string)string{line:=declarationLine(text);if i:=strings.Index(line,"{");i>=0{line=strings.TrimSpace(line[:i])};m:=typeDeclRE.FindStringSubmatch(line);if len(m)==3&&m[2]==name{return strings.TrimSpace(m[1]+" "+name)};return line}
func makeMethodInfo(symbol,owner,name string,m rawMatch)SymbolInfo{sig,ret:=methodParts(m.Text,name);return SymbolInfo{Symbol:symbol,Kind:"METHOD",DeclaringType:owner,Signature:sig,ReturnType:ret,Annotations:annotations(m.Text),Path:m.Path,LineStart:m.StartLine,LineEnd:m.EndLine}}
func makeFieldInfo(symbol,owner,name string,m rawMatch)SymbolInfo{return SymbolInfo{Symbol:symbol,Kind:"FIELD",DeclaringType:owner,Signature:name,ReturnType:fieldType(m.Text,name),Annotations:annotations(m.Text),Path:m.Path,LineStart:m.StartLine,LineEnd:m.EndLine}}
