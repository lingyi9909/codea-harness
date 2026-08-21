package nav

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCallerResultSeparatesConfirmedAndCandidateMatches(t *testing.T) {
	result := CallerResult{
		Symbol: "OrderService.approve",
		Scope:  "src/main/java",
		Callers: []CallerMatch{{CallerSymbol:"OrderFacade.submit", Path:"OrderFacade.java", Line:10, Receiver:"orderService", ReceiverType:"OrderService", Resolution:"CONFIRMED"}},
		Candidates: []CallerMatch{{CallerSymbol:"OrderFacade.submit", Path:"OrderFacade.java", Line:11, Receiver:"unknownService", Resolution:"CANDIDATE"}},
	}
	b, err := json.Marshal(result)
	if err != nil { t.Fatal(err) }
	text := string(b)
	if !strings.Contains(text, `"resolution":"CONFIRMED"`) || !strings.Contains(text, `"candidates"`) || !strings.Contains(text, `"resolution":"CANDIDATE"`) {
		t.Fatalf("unexpected caller contract JSON: %s", text)
	}
}
