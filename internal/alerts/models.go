package alerts

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
)

type (
	AlertConnection = pagination.Connection[Alert]
	AlertEdge       = pagination.Edge[Alert]
)

type Alert interface {
	IsNode()
	IsAlert()
}

type AlertOrder struct {
	Field     AlertOrderField      `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type PrometheusAlert struct {
	ID   ident.Ident `json:"id"`
	Name string      `json:"name"`
}

func (PrometheusAlert) IsNode() {}

func (PrometheusAlert) IsAlert() {}

type AlertOrderField string

const (
	AlertOrderFieldName AlertOrderField = "NAME"
)

var AllAlertOrderField = []AlertOrderField{
	AlertOrderFieldName,
}

func (e AlertOrderField) IsValid() bool {
	switch e {
	case AlertOrderFieldName:
		return true
	}
	return false
}

func (e AlertOrderField) String() string {
	return string(e)
}

func (e *AlertOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = AlertOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid AlertOrderField", str)
	}
	return nil
}

func (e AlertOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
