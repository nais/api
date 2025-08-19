package aiven

import (
	"encoding/json"
	"fmt"
)

type Project struct {
	ID         string `json:"id"`
	VPC        string `json:"vpc"`
	EndpointID string `json:"endpoint_id"`
}

type Projects map[string]Project

var _ json.Unmarshaler = (*Projects)(nil)

func (p *Projects) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	var projects map[string]Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return fmt.Errorf("unmarshalling Aiven projects: %w", err)
	}

	*p = projects
	return nil
}
