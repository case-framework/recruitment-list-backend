package sync

import (
	"encoding/json"
	"fmt"
)

type ConditionType string

const (
	FlagExists      ConditionType = "flagExists"
	FlagHasValue    ConditionType = "flagHasValue"
	FlagNotExists   ConditionType = "flagNotExists"
	FlagNotHasValue ConditionType = "flagNotHasValue"
	HasStatus       ConditionType = "hasStatus"
)

type Condition struct {
	Type  ConditionType `json:"type"`
	Key   string        `json:"key"`
	Value *string       `json:"value,omitempty"`
}

type Operator string

const (
	AND Operator = "AND"
	OR  Operator = "OR"
)

type GroupItem interface {
	isGroupItem()
}

type CriteriaGroup struct {
	Operator   Operator    `json:"operator"`
	Conditions []GroupItem `json:"conditions"`
}

// Implement isGroupItem for both Condition and Group
func (c Condition) isGroupItem()     {}
func (g CriteriaGroup) isGroupItem() {}

// Custom MarshalJSON for Group to handle the interface slice
func (g CriteriaGroup) MarshalJSON() ([]byte, error) {
	type Alias CriteriaGroup
	return json.Marshal(&struct {
		Alias
		Conditions []interface{} `json:"conditions"`
	}{
		Alias:      Alias(g),
		Conditions: interfaceSlice(g.Conditions),
	})
}

func NewCriteriaGroupFromJSON(jsonStr string) (*CriteriaGroup, error) {
	var group CriteriaGroup
	err := json.Unmarshal([]byte(jsonStr), &group)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}
	return &group, nil
}

// Custom UnmarshalJSON for Group to handle the interface slice
func (g *CriteriaGroup) UnmarshalJSON(data []byte) error {
	type Alias CriteriaGroup
	aux := &struct {
		*Alias
		Conditions []json.RawMessage `json:"conditions"`
	}{
		Alias: (*Alias)(g),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	g.Conditions = make([]GroupItem, len(aux.Conditions))
	for i, raw := range aux.Conditions {
		var temp map[string]interface{}
		if err := json.Unmarshal(raw, &temp); err != nil {
			return err
		}
		if _, ok := temp["operator"]; ok {
			var group CriteriaGroup
			if err := json.Unmarshal(raw, &group); err != nil {
				return err
			}
			g.Conditions[i] = group
		} else {
			var condition Condition
			if err := json.Unmarshal(raw, &condition); err != nil {
				return err
			}
			g.Conditions[i] = condition
		}
	}
	return nil
}

// Helper function to convert GroupItem slice to interface{} slice
func interfaceSlice(slice []GroupItem) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}
