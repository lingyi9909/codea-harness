package selection

import "testing"

func TestVerifyJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "valid AUTO_SINGLE",
			json: `{"selectionId":"sel-001","status":"SELECTED","mode":"AUTO_SINGLE","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController"]}`,
		},
		{
			name: "valid USER_MULTI",
			json: `{"selectionId":"sel-002","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController","controller:PaymentController"]}`,
		},
		{
			name:    "SELECTED empty selection",
			json:    `{"selectionId":"sel-003","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":[],"availableControllerIds":["controller:OrderController"]}`,
			wantErr: true,
		},
		{
			name:    "unknown mode",
			json:    `{"selectionId":"sel-004","status":"SELECTED","mode":"UNKNOWN","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController"]}`,
			wantErr: true,
		},
		{
			name:    "selected outside available",
			json:    `{"selectionId":"sel-005","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:PaymentController"],"availableControllerIds":["controller:OrderController"]}`,
			wantErr: true,
		},
		{
			name:    "duplicate selected ids",
			json:    `{"selectionId":"sel-006","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController","controller:OrderController"],"availableControllerIds":["controller:OrderController"]}`,
			wantErr: true,
		},
		{
			name:    "duplicate available ids",
			json:    `{"selectionId":"sel-007","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController","controller:OrderController"]}`,
			wantErr: true,
		},
		{
			name:    "AUTO_SINGLE multiple available",
			json:    `{"selectionId":"sel-008","status":"SELECTED","mode":"AUTO_SINGLE","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController","controller:PaymentController"]}`,
			wantErr: true,
		},
		{
			name: "valid CANCELLED",
			json: `{"selectionId":"sel-009","status":"CANCELLED","mode":"USER_MULTI","selectedControllerIds":[],"availableControllerIds":["controller:OrderController","controller:PaymentController"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyJSON([]byte(tt.json))
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

const changeAnalysisThreeControllers = `{
  "affectedControllers": [
    {"controller":"OrderController","endpoints":["POST /order/approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["OrderController.approve"]},
    {"controller":"PaymentController","endpoints":["POST /payment/pay"],"impactType":"AFFECTED_BY_CALL_CHAIN","sourceSymbols":["PaymentService.pay"]},
    {"controller":"UserController","endpoints":["POST /user/update"],"impactType":"DIRECT_CHANGE","sourceSymbols":["UserController.update"]}
  ]
}`

func TestVerifyAgainstChangeAnalysisRejectsForgedAutoSingle(t *testing.T) {
	selection := `{"selectionId":"sel-1","status":"SELECTED","mode":"AUTO_SINGLE","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController"]}`
	if err := VerifyAgainstChangeAnalysis([]byte(selection), []byte(changeAnalysisThreeControllers)); err == nil {
		t.Fatal("forged AUTO_SINGLE must fail when ChangeAnalysis has 3 controllers")
	}
}

func TestVerifyAgainstChangeAnalysisRejectsMissingAvailableController(t *testing.T) {
	selection := `{"selectionId":"sel-2","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController","controller:PaymentController"]}`
	if err := VerifyAgainstChangeAnalysis([]byte(selection), []byte(changeAnalysisThreeControllers)); err == nil {
		t.Fatal("availableControllerIds missing a real affected controller must fail")
	}
}

func TestVerifyAgainstChangeAnalysisRejectsUnknownAvailableController(t *testing.T) {
	selection := `{"selectionId":"sel-3","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:OrderController"],"availableControllerIds":["controller:OrderController","controller:PaymentController","controller:GhostController"]}`
	if err := VerifyAgainstChangeAnalysis([]byte(selection), []byte(changeAnalysisThreeControllers)); err == nil {
		t.Fatal("availableControllerIds containing unknown controller must fail")
	}
}

func TestVerifyAgainstChangeAnalysisRejectsPartialUserAll(t *testing.T) {
	selection := `{"selectionId":"sel-4","status":"SELECTED","mode":"USER_ALL","selectedControllerIds":["controller:OrderController","controller:PaymentController"],"availableControllerIds":["controller:OrderController","controller:PaymentController","controller:UserController"]}`
	if err := VerifyAgainstChangeAnalysis([]byte(selection), []byte(changeAnalysisThreeControllers)); err == nil {
		t.Fatal("USER_ALL must select every available controller")
	}
}

func TestVerifyAgainstChangeAnalysisRejectsIndirectInDirectOnly(t *testing.T) {
	selection := `{"selectionId":"sel-5","status":"SELECTED","mode":"USER_DIRECT_ONLY","selectedControllerIds":["controller:OrderController","controller:PaymentController"],"availableControllerIds":["controller:OrderController","controller:PaymentController","controller:UserController"]}`
	if err := VerifyAgainstChangeAnalysis([]byte(selection), []byte(changeAnalysisThreeControllers)); err == nil {
		t.Fatal("USER_DIRECT_ONLY must select exactly DIRECT_CHANGE controllers")
	}
}

func TestVerifyAgainstChangeAnalysisAcceptsValidModes(t *testing.T) {
	tests := []string{
		`{"selectionId":"sel-ok-all","status":"SELECTED","mode":"USER_ALL","selectedControllerIds":["controller:OrderController","controller:PaymentController","controller:UserController"],"availableControllerIds":["controller:OrderController","controller:PaymentController","controller:UserController"]}`,
		`{"selectionId":"sel-ok-direct","status":"SELECTED","mode":"USER_DIRECT_ONLY","selectedControllerIds":["controller:OrderController","controller:UserController"],"availableControllerIds":["controller:OrderController","controller:PaymentController","controller:UserController"]}`,
		`{"selectionId":"sel-ok-multi","status":"SELECTED","mode":"USER_MULTI","selectedControllerIds":["controller:PaymentController"],"availableControllerIds":["controller:OrderController","controller:PaymentController","controller:UserController"]}`,
	}
	for _, selection := range tests {
		if err := VerifyAgainstChangeAnalysis([]byte(selection), []byte(changeAnalysisThreeControllers)); err != nil {
			t.Fatalf("valid binding rejected: %v", err)
		}
	}
}
