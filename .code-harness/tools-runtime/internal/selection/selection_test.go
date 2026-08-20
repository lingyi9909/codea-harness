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
