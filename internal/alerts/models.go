package alerts

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type (
	AlertConnection = pagination.Connection[Alert]
	AlertEdge       = pagination.Edge[Alert]
)

type Alert interface {
	model.Node
	GetName() string
	GetEnvironmentName() string
	GetTeamSlug() slug.Slug
	GetState() AlertState
	IsNode()
	IsAlert()
}

type BaseAlert struct {
	Name            string           `json:"name"`
	EnvironmentName string           `json:"environmentName"`
	TeamSlug        slug.Slug        `json:"teamSlug"`
	State           AlertState       `json:"state"`
	Annotations     []*AlertKeyValue `json:"annotations,omitempty"`
	Labels          []*AlertKeyValue `json:"labels,omitempty"`
	Query           string           `json:"query"`
	Duration        float64          `json:"duration"`
}

func (b BaseAlert) GetName() string            { return b.Name }
func (b BaseAlert) GetEnvironmentName() string { return b.EnvironmentName }
func (b BaseAlert) GetTeamSlug() slug.Slug     { return b.TeamSlug }
func (b BaseAlert) GetState() AlertState       { return b.State }

type AlertOrder struct {
	Field     AlertOrderField      `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type AlertOrderField string

const (
	AlertOrderFieldName  AlertOrderField = "NAME"
	AlertOrderFieldState AlertOrderField = "STATE"
)

var AllAlertOrderField = []AlertOrderField{
	AlertOrderFieldName,
	AlertOrderFieldState,
}

func (e AlertOrderField) IsValid() bool {
	switch e {
	case AlertOrderFieldName, AlertOrderFieldState:
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

type (
	PrometheusAlertConnection = pagination.Connection[*PrometheusAlert]
	PrometheusAlertEdge       = pagination.Edge[*PrometheusAlert]
)

type PrometheusAlert struct {
	BaseAlert

	RuleGroup string `json:"ruleGroup"`
}

func (e PrometheusAlert) ID() ident.Ident {
	return newIdent(e.TeamSlug, e.EnvironmentName, e.Name)
}

func (PrometheusAlert) IsNode() {}

func (PrometheusAlert) IsAlert() {}

type PrometheusAlertOrder struct {
	Field     PrometheusAlertOrderField `json:"field"`
	Direction model.OrderDirection      `json:"direction"`
}

type PrometheusAlertOrderField string

const (
	PrometheusAlertOrderFieldName  PrometheusAlertOrderField = "NAME"
	PrometheusAlertOrderFieldState PrometheusAlertOrderField = "STATE"
)

var AllPrometheusAlertOrderField = []PrometheusAlertOrderField{
	PrometheusAlertOrderFieldName,
	PrometheusAlertOrderFieldState,
}

func (e PrometheusAlertOrderField) IsValid() bool {
	switch e {
	case PrometheusAlertOrderFieldName, PrometheusAlertOrderFieldState:
		return true
	}
	return false
}

func (e PrometheusAlertOrderField) String() string {
	return string(e)
}

func (e *PrometheusAlertOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = PrometheusAlertOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PrometheusAlertOrderField", str)
	}
	return nil
}

func (e PrometheusAlertOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamAlertsFilter struct {
	States []AlertState `json:"states,omitempty"`
}

type AlertState string

const (
	AlertStateFiring   AlertState = "FIRING"
	AlertStateInactive AlertState = "INACTIVE"
	AlertStatePending  AlertState = "PENDING"
)

var AllAlertState = []AlertState{
	AlertStateFiring,
	AlertStateInactive,
	AlertStatePending,
}

func (e AlertState) IsValid() bool {
	switch e {
	case AlertStateFiring, AlertStateInactive, AlertStatePending:
		return true
	}
	return false
}

func (e AlertState) String() string {
	return string(e)
}

func (e *AlertState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = AlertState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid AlertState", str)
	}
	return nil
}

func (e AlertState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type AlertKeyValue struct {
	// The key for the label or annotation.
	Key string `json:"key"`
	// The value for the label or annotation.
	Value string `json:"value"`
}
