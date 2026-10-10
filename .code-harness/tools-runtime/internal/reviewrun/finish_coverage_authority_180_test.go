package reviewrun

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

const externalSchemaNote180 = "数据库表结构（orders 表实际列名，如 tenant_id/id/status）不在本次授权读取范围内，修复建议中的列名需以实际 schema 为准。"

// A complete, real Controller->Service->Mapper->SQL chain MUST NOT be
// downgraded by the model explaining that external DB DDL was not reviewed.
// Confirmed CRITICAL/HIGH findings retain their blocking conclusion. Both
// kinds of gaps stay distinguishable in result.json and in the durable report.
func TestT5CompleteCriticalAndExternalSchemaNoteRemainsBlocking(t *testing.T) {
    root, started := mustRealPreparedRun180(t)
    runDir := filepath.Dir(started.ReportPath)
    scope, err := loadScope180(runDir, started.RunID)
    if err != nil {t.Fatal(err)}
    if scope.Coverage != "COMPLETE" || len(scope.Chains) != 1 ||
        len(scope.Chains[0].Unresolved) != 0 {t.Fatalf("expected proven complete chain: %+v", scope)}

    var ref ReadRef
    for _, candidate := range scope.Reads {
        if candidate.Path == "src/main/java/com/example/OrderServiceImpl.java" {
            ref = candidate
            break
        }
    }
    if ref.Path == "" {t.Fatalf("missing service evidence in real scope: %+v", scope.Reads)}

    severities := []string{"CRITICAL", "HIGH", "MEDIUM"}
    findings := make([]Finding, 0, len(severities))
    for i, sev := range severities {
        findings = append(findings, Finding{
            ID: "F" + string(rune('1'+i)),
            Severity: sev,
            Problem: "real selected service method has a risky update path",
            Impact: "order state can be changed without authorization",
            Recommendation: "validate state and tenant before data update",
            Verification: "review the selected service and mapper code",
            Evidence: []Evidence{{Ref: ref, Quote: "public void create()"}},
        })
    }

    got, err := Finish(context.Background(), root, FinishRequest{
        RunID: started.RunID, Reads: scope.Reads,
        Findings: findings, Gaps: []string{externalSchemaNote180},
    })
    if err != nil {t.Fatal(err)}
    if got.Execution != "COMPLETE" || got.Coverage != "COMPLETE" ||
        got.ReviewConclusion != "BLOCKING" {
        t.Fatalf("explanatory schema note incorrectly downgraded high-risk review: %+v",got)
    }

    raw, err := os.ReadFile(filepath.Join(runDir,"result.json"))
    if err != nil {t.Fatal(err)}
    var result resultEnvelope
    if err:=json.Unmarshal(raw,&result);err!=nil {t.Fatal(err)}
    if result.Coverage!="COMPLETE" || result.ReviewConclusion!="BLOCKING" ||
        len(result.Findings)!=3 || len(result.CoverageGaps)!=0 ||
        len(result.InformationalGaps)!=1 || result.InformationalGaps[0]!=externalSchemaNote180 ||
        len(result.Gaps)!=1 || result.Gaps[0]!=externalSchemaNote180 {
        t.Fatalf("result lost notes or conflated gap authority: %+v",result)
    }
    report,err:=os.ReadFile(started.ReportPath)
    if err!=nil {t.Fatal(err)}
    for _,want:=range []string{"存在阻断问题","覆盖说明：","模型提示的外部信息",externalSchemaNote180} {
        if !strings.Contains(string(report),want) {
            t.Fatalf("report missing %q:\n%s",want,report)
        }
    }
    if strings.Contains(string(report),"补齐覆盖缺口") {
        t.Fatalf("external schema note falsely reported as coverage defect:\n%s",report)
    }
}

// A genuinely unresolved Runtime chain remains PARTIAL->UNDETERMINED even
// when the model finds a CRITICAL issue and adds an explanatory note.
func TestT5ActualUnresolvedChainRemainsUndeterminedWithCriticalFinding(t *testing.T) {
    root,started,ref:=setupT3Scope180(t,true)
    got,err:=Finish(context.Background(),root,FinishRequest{
        RunID: started.RunID, Reads: []ReadRef{ref},
        Findings: []Finding{{
            ID:"F-critical",Severity:"CRITICAL",
            Problem:"unsafe entrypoint",Impact:"unauthorized write",
            Recommendation:"restrict access",Verification:"review callsite",
            Evidence: []Evidence{{Ref:ref,Quote:"class A { void entry() {} }"}},
        }},
        Gaps: []string{externalSchemaNote180},
    })
    if err!=nil {t.Fatal(err)}
    if got.Coverage!="PARTIAL" || got.ReviewConclusion!="UNDETERMINED" {
        t.Fatalf("true unresolved Runtime chain must never be BLOCKING/complete: %+v",got)
    }
    b,err:=os.ReadFile(filepath.Join(filepath.Dir(started.ReportPath),"result.json"))
    if err!=nil {t.Fatal(err)}
    var result resultEnvelope
    if err:=json.Unmarshal(b,&result);err!=nil {t.Fatal(err)}
    if len(result.CoverageGaps)!=1 || result.CoverageGaps[0]!="A.entry receiver unresolved" ||
        len(result.InformationalGaps)!=1 || result.InformationalGaps[0]!=externalSchemaNote180 ||
        len(result.Gaps)!=2 || len(result.Findings)!=1 {
        t.Fatalf("runtime coverage gap not distinct from explanatory note: %+v",result)
    }
    report,err:=os.ReadFile(started.ReportPath)
    if err!=nil {t.Fatal(err)}
    for _,want:=range []string{"Runtime 已确认的调用链覆盖缺口","A.entry receiver unresolved",
        "模型提示的外部信息",externalSchemaNote180} {
        if !strings.Contains(string(report),want) {
            t.Fatalf("partial report missing %q:\n%s",want,report)
        }
    }
}
