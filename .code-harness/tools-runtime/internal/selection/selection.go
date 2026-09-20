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

type changeAnalysis struct {
	AffectedControllers []affectedController `json:"affectedControllers"`
}

type affectedController struct {
	Controller string `json:"controller"`
	ImpactType string `json:"impactType"`
}

func VerifyAgainstChangeAnalysis(selectionJSON, changeAnalysisJSON []byte) error {
	if err := VerifyJSON(selectionJSON); err != nil {
		return err
	}

	var a artifact
	if err := json.Unmarshal(selectionJSON, &a); err != nil {
		return fmt.Errorf("invalid test target selection: %w", err)
	}
	var ca changeAnalysis
	if err := json.Unmarshal(changeAnalysisJSON, &ca); err != nil {
		return fmt.Errorf("invalid change analysis: %w", err)
	}

	expected := make([]string, 0, len(ca.AffectedControllers))
	direct := make([]string, 0, len(ca.AffectedControllers))
	for _, controller := range ca.AffectedControllers {
		if controller.Controller == "" {
			return errors.New("change analysis affected controller must not be empty")
		}
		id := "controller:" + controller.Controller
		expected = append(expected, id)
		if controller.ImpactType == "DIRECT_CHANGE" {
			direct = append(direct, id)
		}
	}

	if !sameStringSet(a.AvailableControllerIDs, expected) {
		return errors.New("availableControllerIds must exactly match ChangeAnalysis.affectedControllers")
	}
	if a.Status != StatusSelected {
		return nil
	}

	switch a.Mode {
	case "AUTO_SINGLE":
		if len(expected) != 1 {
			return errors.New("AUTO_SINGLE is allowed only when ChangeAnalysis has exactly one affected controller")
		}
	case "USER_ALL":
		if !sameStringSet(a.SelectedControllerIDs, expected) {
			return errors.New("USER_ALL must select all affected controllers")
		}
	case "USER_DIRECT_ONLY":
		if !sameStringSet(a.SelectedControllerIDs, direct) {
			return errors.New("USER_DIRECT_ONLY must select exactly DIRECT_CHANGE controllers")
		}
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

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[string]struct{}, len(left))
	for _, value := range left {
		set[value] = struct{}{}
	}
	if len(set) != len(left) {
		return false
	}
	for _, value := range right {
		if _, ok := set[value]; !ok {
			return false
		}
	}
	return true
}
