package reviewcontext

import (
	"reflect"
	"testing"
)

func Test170MyBatisGeneratedParameterAliasesRequireNumericForm(t *testing.T) {
	known := map[string]bool{"id": true}
	missing := myBatisMissingParams170(known, []byte("#{param1} #{param2} #{arg0} #{arg1} #{parameterCode} #{argument} #{param0} #{arg01}"))
	want := []string{"arg01", "argument", "param0", "parameterCode"}
	if !reflect.DeepEqual(missing, want) {
		t.Fatalf("generated aliases must require their exact numeric form: got=%v want=%v", missing, want)
	}
}
