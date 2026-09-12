package reviewcontext

import (
	"reflect"
	"testing"
)

func Test170MyBatisGeneratedAliasesRespectActualParameterCount(t *testing.T) {
	oneParam := map[string]bool{"id": true}
	missing := myBatisMissingParams170(oneParam, []byte("#{param1} #{param2} #{arg0} #{arg1} #{parameterCode} #{argument} #{param0} #{arg01}"))
	want := []string{"arg0", "arg01", "arg1", "argument", "param0", "param2", "parameterCode"}
	if !reflect.DeepEqual(missing, want) {
		t.Fatalf("one-parameter aliases must be bounded by the actual contract: got=%v want=%v", missing, want)
	}

	twoParams := map[string]bool{"id": true, "status": true}
	missing = myBatisMissingParams170(twoParams, []byte("#{param1} #{param2} #{param3}"))
	want = []string{"param3"}
	if !reflect.DeepEqual(missing, want) {
		t.Fatalf("two-parameter aliases must allow only param1..param2: got=%v want=%v", missing, want)
	}
}
