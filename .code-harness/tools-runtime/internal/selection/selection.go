package selection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	StatusSelected  = "SELECTED"
	StatusCancelled = "CANCELLED"
)

var validModes = map[string]struct{}{
	"AUTO_SINGLE":       {},
	"USER_MULTI":        {},
	"USER_ALL":          {},
	"USER_DIRECT_ONLY":  {},
	"FALLBACK_NUMBERED": {},
}

type artifact struct {
	SelectionID            string   `json:"selectionId"`
	Status                 string   `json:"status"`
	Mode                   string   `json:"mode"`
	SelectedControllerIDs  []string `json:"selectedControllerIds"`
	AvailableControllerIDs []string `json:"availableControllerIds"`
}

func VerifyJSON(data []byte) error {
	var a artifact
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return fmt.Errorf("invalid test target selection: %w", err)
	}
	if a.SelectionID == "" {
		return errors.New("selectionId must not be empty")
	}
	if a.Status != StatusSelected && a.Status != StatusCancelled {
		return fmt.Errorf("unknown selection status %q", a.Status)
	}
	if _, ok := validModes[a.Mode]; !ok {
		return fmt.Errorf("unknown selection mode %q", a.Mode)
	}
	if len(a.AvailableControllerIDs) == 0 {
		return errors.New("availableControllerIds must not be empty")
	}
	if err := requireUnique("availableControllerIds", a.AvailableControllerIDs); err != nil {
		return err
	}
	if err := requireUnique("selectedControllerIds", a.SelectedControllerIDs); err != nil {
		return err
	}

	available := make(map[string]struct{}, len(a.AvailableControllerIDs))
	for _, id := range a.AvailableControllerIDs {
		if id == "" {
			return errors.New("availableControllerIds must not contain empty ids")
		}
		available[id] = struct{}{}
	}
	for _, id := range a.SelectedControllerIDs {
		if id == "" {
			return errors.New("selectedControllerIds must not contain empty ids")
		}
		if _, ok := available[id]; !ok {
			return fmt.Errorf("selected controller %q is not available", id)
		}
	}

	if a.Mode == "AUTO_SINGLE" {
		if a.Status != StatusSelected || len(a.AvailableControllerIDs) != 1 || len(a.SelectedControllerIDs) != 1 {
			return errors.New("AUTO_SINGLE requires SELECTED status with exactly one available and one selected controller")
		}
	}
	if a.Status == StatusSelected && len(a.SelectedControllerIDs) == 0 {
		return errors.New("SELECTED requires at least one selected controller")
	}
	return nil
}

func requireUnique(name string, ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%s contains duplicate id %q", name, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
