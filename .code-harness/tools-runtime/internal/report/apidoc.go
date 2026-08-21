package report

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "codea-harness-tools/internal/schema"
)

type ApiDocRequest struct {
    RunID          string      `json:"runId"`
    HarnessVersion string      `json:"harnessVersion"`
    ApiDoc         ApiDocument `json:"apiDoc"`
}

type ApiDocument struct {
    Controllers []ApiController `json:"controllers"`
}

type ApiController struct {
    Name string        `json:"name"`
    APIs []ApiEndpoint `json:"apis"`
}

type ApiEndpoint struct {
    Title           string              `json:"title"`
    HTTPMethod      string              `json:"httpMethod"`
    Path            string              `json:"path"`
    Description     string              `json:"description"`
    Request         ApiRequestShape     `json:"request"`
    Response        ApiResponseShape    `json:"response"`
    Permissions     []SemanticStatement `json:"permissions"`
    Preconditions   []SemanticStatement `json:"preconditions"`
    BusinessFlow    []SemanticStatement `json:"businessFlow"`
    StateTransitions []SemanticStatement `json:"stateTransitions"`
    DataEffects     []SemanticStatement `json:"dataEffects"`
    ExternalEffects []SemanticStatement `json:"externalEffects"`
    Transactions    []SemanticStatement `json:"transactions"`
    Idempotency     []SemanticStatement `json:"idempotency"`
    ErrorCodes      []ApiErrorCode      `json:"errorCodes"`
    TestCoverage    []SemanticStatement `json:"testCoverage"`
    BusinessNotes   []SemanticStatement `json:"businessNotes"`
}

type ApiRequestShape struct {
    Fields  []ApiRequestField `json:"fields"`
    Example any               `json:"example"`
}

type ApiResponseShape struct {
    Fields  []ApiResponseField `json:"fields"`
    Example any                `json:"example"`
}

type ApiRequestField struct {
    Name        string            `json:"name"`
    Type        string            `json:"type"`
    Required    bool              `json:"required"`
    Description string            `json:"description"`
    Validation  []string          `json:"validation"`
    EnumValues  []ApiEnumValue    `json:"enumValues"`
    Fields      []ApiRequestField `json:"fields,omitempty"`
}

type ApiResponseField struct {
    Name        string             `json:"name"`
    Type        string             `json:"type"`
    Description string             `json:"description"`
    EnumValues  []ApiEnumValue     `json:"enumValues"`
    Fields      []ApiResponseField `json:"fields,omitempty"`
}

type ApiEnumValue struct {
    Value       any    `json:"value"`
    Description string `json:"description"`
}

type SemanticStatement struct {
    Statement string   `json:"statement"`
    Status    string   `json:"status"`
    Evidence  []string `json:"evidence"`
}

type ApiErrorCode struct {
    Code     any      `json:"code"`
    Scenario string   `json:"scenario"`
    Status   string   `json:"status"`
    Evidence []string `json:"evidence"`
}

func DecodeApiDocRequest(data []byte) (ApiDocRequest, error) {
    var req ApiDocRequest
    dec := json.NewDecoder(bytes.NewReader(data))
    dec.UseNumber()
    dec.DisallowUnknownFields()
    if err := dec.Decode(&req); err != nil {
        return ApiDocRequest{}, fmt.Errorf("decode api-doc report request: %w", err)
    }
    var extra any
    if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
        if err == nil {
            return ApiDocRequest{}, errors.New("decode api-doc report request: multiple JSON values are not allowed")
        }
        return ApiDocRequest{}, fmt.Errorf("decode api-doc report request: %w", err)
    }
    if err := validateApiDocRequest(req); err != nil {
        return ApiDocRequest{}, err
    }
    return req, nil
}

func validateApiDocRequest(req ApiDocRequest) error {
    if !validArtifactID(req.RunID) {
        return errors.New("invalid api-doc report runId")
    }
    if strings.TrimSpace(req.HarnessVersion) == "" {
        return errors.New("api-doc report requires harnessVersion")
    }
    if len(req.ApiDoc.Controllers) == 0 {
        return errors.New("api-doc report requires at least one controller")
    }
    allowedMethod := map[string]bool{"GET":true,"POST":true,"PUT":true,"DELETE":true,"PATCH":true,"OPTIONS":true,"HEAD":true}
    for ci, c := range req.ApiDoc.Controllers {
        if strings.TrimSpace(c.Name) == "" || len(c.APIs) == 0 {
            return fmt.Errorf("controller %d requires name and apis", ci)
        }
        for ai, a := range c.APIs {
            if strings.TrimSpace(a.Title) == "" || !allowedMethod[a.HTTPMethod] || !strings.HasPrefix(a.Path, "/") {
                return fmt.Errorf("controller %q api %d has invalid title/httpMethod/path", c.Name, ai)
            }
            for _, group := range [][]SemanticStatement{a.Permissions,a.Preconditions,a.BusinessFlow,a.StateTransitions,a.DataEffects,a.ExternalEffects,a.Transactions,a.Idempotency,a.TestCoverage,a.BusinessNotes} {
                for _, s := range group {
                    if err := validateSemantic(s); err != nil { return err }
                }
            }
            for _, e := range a.ErrorCodes {
                if strings.TrimSpace(e.Scenario) == "" { return errors.New("api-doc error code requires scenario") }
                if err := validateSemantic(SemanticStatement{Statement:e.Scenario,Status:e.Status,Evidence:e.Evidence}); err != nil { return err }
            }
        }
    }
    return nil
}

func validateSemantic(s SemanticStatement) error {
    if strings.TrimSpace(s.Statement) == "" { return errors.New("api-doc semantic statement is empty") }
    switch s.Status {
    case "CONFIRMED", "INFERRED":
        if len(s.Evidence) == 0 { return fmt.Errorf("api-doc %s statement requires evidence", s.Status) }
    case "UNKNOWN":
    default:
        return fmt.Errorf("invalid api-doc semantic status %q", s.Status)
    }
    return nil
}

func RenderApiDoc(req ApiDocRequest) (string, error) {
    if err := validateApiDocRequest(req); err != nil { return "", err }
    var b strings.Builder
    fmt.Fprintln(&b, "# Frontend API Documentation")
    fmt.Fprintf(&b, "Harness Version: %s\n", singleLine(req.HarnessVersion))
    for _, c := range req.ApiDoc.Controllers {
        fmt.Fprintf(&b, "\n## %s\n", singleLine(c.Name))
        for _, a := range c.APIs {
            fmt.Fprintf(&b, "\n### %s %s\n", a.HTTPMethod, singleLine(a.Path))
            fmt.Fprintf(&b, "**%s**\n\n", singleLine(a.Title))
            if strings.TrimSpace(a.Description) != "" { fmt.Fprintln(&b, strings.TrimSpace(a.Description)) }
            renderRequest(&b, a.Request)
            renderResponse(&b, a.Response)
            renderSemanticSection(&b, "Permissions", a.Permissions)
            renderSemanticSection(&b, "Preconditions", a.Preconditions)
            renderSemanticSection(&b, "Business Flow", a.BusinessFlow)
            renderSemanticSection(&b, "State Transitions", a.StateTransitions)
            renderSemanticSection(&b, "Data Effects", a.DataEffects)
            renderSemanticSection(&b, "External Effects", a.ExternalEffects)
            renderSemanticSection(&b, "Transactions", a.Transactions)
            renderSemanticSection(&b, "Idempotency", a.Idempotency)
            renderErrors(&b, a.ErrorCodes)
            renderSemanticSection(&b, "Test Coverage", a.TestCoverage)
            renderSemanticSection(&b, "Business Notes", a.BusinessNotes)
        }
    }
    return b.String(), nil
}

func renderRequest(b *strings.Builder, r ApiRequestShape) {
    fmt.Fprintln(b, "\n#### Request")
    if len(r.Fields) == 0 { fmt.Fprintln(b, "无") } else {
        fmt.Fprintln(b, "| Field | Type | Required | Validation | Description |")
        fmt.Fprintln(b, "|---|---|---|---|---|")
        writeRequestFields(b, r.Fields, "")
    }
    renderExample(b, "Request Example", r.Example)
}

func writeRequestFields(b *strings.Builder, fields []ApiRequestField, prefix string) {
    for _, f := range fields {
        name:=f.Name; if prefix!="" { name=prefix+"."+f.Name }
        fmt.Fprintf(b,"| %s | %s | %t | %s | %s |\n", table(name),table(f.Type),f.Required,table(strings.Join(f.Validation, ", ")),table(f.Description))
        if len(f.EnumValues)>0 { fmt.Fprintf(b,"| %s enum |  |  | %s |  |\n",table(name),table(enumText(f.EnumValues))) }
        writeRequestFields(b,f.Fields,name)
    }
}

func renderResponse(b *strings.Builder, r ApiResponseShape) {
    fmt.Fprintln(b, "\n#### Response")
    if len(r.Fields) == 0 { fmt.Fprintln(b, "无") } else {
        fmt.Fprintln(b, "| Field | Type | Description |")
        fmt.Fprintln(b, "|---|---|---|")
        writeResponseFields(b,r.Fields,"")
    }
    renderExample(b,"Response Example",r.Example)
}

func writeResponseFields(b *strings.Builder, fields []ApiResponseField, prefix string) {
    for _, f := range fields {
        name:=f.Name; if prefix!="" { name=prefix+"."+f.Name }
        desc:=f.Description; if len(f.EnumValues)>0 { if desc!="" { desc+="; " }; desc+="Enum: "+enumText(f.EnumValues) }
        fmt.Fprintf(b,"| %s | %s | %s |\n",table(name),table(f.Type),table(desc))
        writeResponseFields(b,f.Fields,name)
    }
}

func enumText(values []ApiEnumValue) string {
    parts:=make([]string,0,len(values))
    for _,v:=range values { parts=append(parts,fmt.Sprintf("%v (%s)",v.Value,singleLine(v.Description))) }
    return strings.Join(parts,", ")
}

func renderExample(b *strings.Builder,title string,v any) {
    if v==nil { return }
    data,err:=json.MarshalIndent(v,"","  "); if err!=nil { return }
    if string(data)=="null" { return }
    fmt.Fprintf(b,"\n**%s**\n\n```json\n%s\n```\n",title,string(data))
}

func renderSemanticSection(b *strings.Builder,title string,values []SemanticStatement) {
    if len(values)==0 { return }
    fmt.Fprintf(b,"\n#### %s\n",title)
    for _,s:=range values {
        fmt.Fprintf(b,"- [%s] %s",s.Status,singleLine(s.Statement))
        if len(s.Evidence)>0 { fmt.Fprintf(b," — Evidence: %s",strings.Join(cleanLines(s.Evidence),"; ")) }
        fmt.Fprintln(b)
    }
}

func renderErrors(b *strings.Builder,values []ApiErrorCode) {
    if len(values)==0 { return }
    fmt.Fprintln(b,"\n#### Error Codes")
    fmt.Fprintln(b,"| Code | Scenario | Status | Evidence |")
    fmt.Fprintln(b,"|---|---|---|---|")
    for _,e:=range values { fmt.Fprintf(b,"| %s | %s | %s | %s |\n",table(fmt.Sprint(e.Code)),table(e.Scenario),e.Status,table(strings.Join(cleanLines(e.Evidence),"; "))) }
}

func cleanLines(values []string) []string { out:=make([]string,0,len(values)); for _,v:=range values { out=append(out,singleLine(v)) }; return out }
func table(v string) string { return strings.ReplaceAll(singleLine(v),"|","\\|") }

func WriteApiDocRequestFile(repoRoot, inputPath string) (string, error) {
    root, err := filepath.Abs(repoRoot); if err != nil { return "",err }
    inputAbs:=inputPath; if !filepath.IsAbs(inputAbs) { inputAbs=filepath.Join(root,inputAbs) }
    inputAbs,err=filepath.Abs(inputAbs); if err!=nil { return "",err }
    if !strings.EqualFold(filepath.Ext(inputAbs),".json") { return "",errors.New("api-doc report input must be JSON") }
    data,err:=os.ReadFile(inputAbs); if err!=nil { return "",fmt.Errorf("read api-doc report request: %w",err) }
    req,err:=DecodeApiDocRequest(data); if err!=nil { return "",err }

    requestRoot:=filepath.Join(root,".code-harness","runs",req.RunID,"requests")
    rel,err:=filepath.Rel(requestRoot,inputAbs); if err!=nil || rel=="." || rel==".." || strings.HasPrefix(rel,".."+string(filepath.Separator)) { return "",errors.New("api-doc report input must be under .code-harness/runs/<runId>/requests") }

    schemaBytes,err:=os.ReadFile(filepath.Join(root,".code-harness","contracts","api-doc.schema.json")); if err!=nil { return "",fmt.Errorf("read api-doc schema: %w",err) }
    apiDocJSON,err:=json.Marshal(req.ApiDoc); if err!=nil { return "",err }
    if err:=schema.ValidateJSON(schemaBytes,apiDocJSON); err!=nil { return "",fmt.Errorf("validate api-doc contract: %w",err) }

    markdown,err:=RenderApiDoc(req); if err!=nil { return "",err }
    runDir:=filepath.Join(root,".code-harness","runs",req.RunID)
    if err:=os.MkdirAll(runDir,0o755); err!=nil { return "",fmt.Errorf("create api-doc report directory: %w",err) }
    path:=filepath.Join(runDir,"api-doc.md")
    if err:=os.WriteFile(path,[]byte(markdown),0o600); err!=nil { return "",fmt.Errorf("write api-doc report: %w",err) }
    if err:=os.Remove(inputAbs); err!=nil { return "",fmt.Errorf("remove api-doc transport after success: %w",err) }
    return path,nil
}
