package schema

import (
	"strings"
	"testing"
)

func TestConfirmedBusinessSemanticCannotPassWithoutEvidence(t *testing.T) {
	input := strings.Replace(string(validAPIDocJSON()), `"evidence":["OrderServiceImpl.java:128"]`, `"evidence":[]`, 1)
	if err := ValidateJSON(loadAPIDocSchema(t), []byte(input)); err == nil {
		t.Fatal("CONFIRMED semantic without evidence must fail")
	}
}

func TestEmptyStringErrorCodeFails(t *testing.T) {
	input := strings.Replace(string(validAPIDocJSON()), `"code":"ORDER_NOT_FOUND"`, `"code":""`, 1)
	if err := ValidateJSON(loadAPIDocSchema(t), []byte(input)); err == nil {
		t.Fatal("empty string error code must fail")
	}
}
