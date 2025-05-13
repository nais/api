package servicemaintenance

import (
	"fmt"
	"io"
	"strconv"
)

type ServiceType string

const (
	ServiceTypeOpensearch ServiceType = "OPENSEARCH"
	ServiceTypeValkey     ServiceType = "VALKEY"
)

var AllServiceType = []ServiceType{
	ServiceTypeOpensearch,
	ServiceTypeValkey,
}

func (e ServiceType) IsValid() bool {
	switch e {
	case ServiceTypeOpensearch, ServiceTypeValkey:
		return true
	}
	return false
}

func (e ServiceType) String() string {
	return string(e)
}

func (e *ServiceType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ServiceType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ServiceType", str)
	}
	return nil
}

func (e ServiceType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
