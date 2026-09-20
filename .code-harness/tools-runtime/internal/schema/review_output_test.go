package schema

import (
	"os"
	"testing"
)

func TestReviewOutputSchemaRequiresProblemAndCategory(t *testing.T) {
	schemaBytes, err := os.ReadFile("../../../contracts/review-output.schema.json")
	if err != nil { t.Fatal(err) }

	valid := []byte(`{
	  "summary":"发现订单状态迁移问题",
	  "findings":[{
	    "id":"F-001",
	    "category":"PRODUCTION_CODE",
	    "severity":"high",
	    "file":"src/main/java/OrderServiceImpl.java",
	    "line":128,
	    "problem":"订单状态变更缺少当前状态校验。",
	    "evidence":"approve() 直接更新状态。",
	    "impact":"可能产生非法状态迁移。",
	    "recommendation":"更新前校验当前状态。",
	    "needsTest":true,
	    "introducedByChange":true,
	    "confidence":0.95
	  }]
	}`)
	if err := ValidateJSON(schemaBytes, valid); err != nil {
		t.Fatalf("valid review output rejected: %v", err)
	}

	missingProblem := []byte(`{
	  "summary":"发现问题",
	  "findings":[{
	    "id":"F-001","category":"PRODUCTION_CODE","severity":"high",
	    "file":"src/main/java/A.java","line":1,
	    "evidence":"证据","impact":"影响","recommendation":"建议",
	    "needsTest":true,"introducedByChange":true,"confidence":0.9
	  }]
	}`)
	if err := ValidateJSON(schemaBytes, missingProblem); err == nil {
		t.Fatal("missing problem must be rejected")
	}

	invalidCategory := []byte(`{
	  "summary":"发现问题",
	  "findings":[{
	    "id":"F-001","category":"TEST_QUALITY","severity":"high",
	    "file":"src/test/java/A.java","line":1,"problem":"问题",
	    "evidence":"证据","impact":"影响","recommendation":"建议",
	    "needsTest":true,"introducedByChange":true,"confidence":0.9
	  }]
	}`)
	if err := ValidateJSON(schemaBytes, invalidCategory); err == nil {
		t.Fatal("ordinary test quality category must be rejected")
	}
}
