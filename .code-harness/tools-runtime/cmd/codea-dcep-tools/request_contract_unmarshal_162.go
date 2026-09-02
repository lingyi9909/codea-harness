package main

import (
	"encoding/json"
	"fmt"

	"codea-harness-tools/internal/requestcontract"
)

func (r *analysisSnapshotRequest162) UnmarshalJSON(data []byte) error {
	if err := requestcontract.Validate("change-set-request.schema.json", data); err != nil {
		return fmt.Errorf("CHANGE_SET_REQUEST_SCHEMA_INVALID: %w", err)
	}
	type alias analysisSnapshotRequest162
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = analysisSnapshotRequest162(decoded)
	return nil
}

func (r *analysisInventoryRequest153) UnmarshalJSON(data []byte) error {
	if err := requestcontract.Validate("analysis-inventory-request.schema.json", data); err != nil {
		return fmt.Errorf("ANALYSIS_INVENTORY_REQUEST_SCHEMA_INVALID: %w", err)
	}
	type alias analysisInventoryRequest153
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = analysisInventoryRequest153(decoded)
	return nil
}

func (r *reviewOptionsRequest) UnmarshalJSON(data []byte) error {
	if err := requestcontract.Validate("review-options-request.schema.json", data); err != nil {
		return fmt.Errorf("REVIEW_OPTIONS_REQUEST_SCHEMA_INVALID: %w", err)
	}
	type alias reviewOptionsRequest
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = reviewOptionsRequest(decoded)
	return nil
}
