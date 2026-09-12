package reviewcontext

import (
	"reflect"
	"testing"
)

func Test170MyBatisGeneratedAliasesRespectActualParameterCount(t *testing.T) {
	known := map[string]bool{"id": true}
	missing := myBatisMissingParams170(known, []byte("#{param1} #{param2} #{arg0} #{arg1} #{parameterCode} #{argument} #{param0} #{arg01}"))
	want := []string{"arg0", "arg01", "arg1", "argument", "param0", "param2", "parameterCode"}
	if !reflect.DeepEqual(missing, want) {
		t.Fatalf("generated aliases must be bounded by the actual parameter contract: got=%v want=%v", missing, want)
	}
}
