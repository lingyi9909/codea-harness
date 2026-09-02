package analysis

import (
	"encoding/json"
	"fmt"

	"codea-harness-tools/internal/requestcontract"
)

func (r *CertifyRequest) UnmarshalJSON(data []byte) error {
	if err := requestcontract.Validate("analysis-certify-request.schema.json", data); err != nil {
		return fmt.Errorf("ANALYSIS_CERTIFY_REQUEST_SCHEMA_INVALID: %w", err)
	}
	type alias CertifyRequest
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = CertifyRequest(decoded)
	return nil
}
