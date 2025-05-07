package price

import (
	"fmt"
	"io"
	"strconv"
)

type Price struct {
	Description string       `json:"description"`
	Price       float64      `json:"price"`
	Unit        string       `json:"unit"`
	Currency    string       `json:"currency"`
	Type        ResourceType `json:"type"`
}

type ResourceType string

const (
	ResourceTypeCPU    ResourceType = "CPU"
	ResourceTypeMemory ResourceType = "MEMORY"
)

var AllResourceType = []ResourceType{
	ResourceTypeCPU,
	ResourceTypeMemory,
}

func (e ResourceType) IsValid() bool {
	switch e {
	case ResourceTypeCPU, ResourceTypeMemory:
		return true
	}
	return false
}

func (e ResourceType) String() string {
	return string(e)
}

func (e *ResourceType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ResourceType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ResourceType", str)
	}
	return nil
}

func (e ResourceType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
