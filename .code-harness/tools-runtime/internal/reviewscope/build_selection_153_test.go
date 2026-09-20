package reviewscope

import (
	"reflect"
	"testing"
)

const task153TwoUpstreamServiceAnalysis = `{
  "changedFiles":[
    {"path":"src/main/java/AController.java","role":"Controller"},
    {"path":"src/main/java/BController.java","role":"Controller"},
    {"path":"src/main/java/CommonService.java","role":"Service"}
  ],
  "callChains":[
    {"entryPoint":"AController.submit","chain":["AController.submit","CommonService.execute"]},
    {"entryPoint":"BController.submit","chain":["BController.submit","CommonService.execute"]}
  ],
  "symbolLocations":[
    {"workspace":"current","symbol":"AController.submit","path":"src/main/java/AController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"workspace":"current","symbol":"BController.submit","path":"src/main/java/BController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"workspace":"current","symbol":"CommonService.execute","path":"src/main/java/CommonService.java","role":"Service","source":"FIND_SYMBOL"}
  ],
  "resourceRelations":[],
  "reviewCoverage":{
    "reviewedFiles":[
      {"path":"src/main/java/AController.java","role":"Controller"},
      {"path":"src/main/java/BController.java","role":"Controller"},
      {"path":"src/main/java/CommonService.java","role":"Service"}
    ],
    "unresolvedSymbols":[]
  }
}`

func Test153BuildTargetedSelectionKeepsAllSelectedUpstreamChainsForService(t *testing.T) {
	selected := []CallChain{
		{EntryPoint: "AController.submit", Chain: []string{"AController.submit", "CommonService.execute"}},
		{EntryPoint: "BController.submit", Chain: []string{"BController.submit", "CommonService.execute"}},
	}
	got, err := BuildTargetedSelection(selected, []byte(task153TwoUpstreamServiceAnalysis))
	if err != nil { t.Fatal(err) }
	if got.Mode != "TARGETED" || got.Target == nil || got.Target.Symbol != "CommonService.execute" || got.Target.Kind != "METHOD" {
		t.Fatalf("service convergence must stay TARGETED at common downstream symbol: %+v", got)
	}
	if !reflect.DeepEqual(got.SelectedCallChains, selected) {
		t.Fatalf("Runtime must preserve both selected upstream chains: %+v", got.SelectedCallChains)
	}
	wantFiles := []string{"src/main/java/AController.java", "src/main/java/BController.java", "src/main/java/CommonService.java"}
	if !reflect.DeepEqual(got.ScopedFiles, wantFiles) {
		t.Fatalf("multi-upstream scope must be exact union, got=%v want=%v", got.ScopedFiles, wantFiles)
	}
}
