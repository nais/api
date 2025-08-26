package alerts

import (
	"fmt"
	"io"
	"strconv"
	"time"

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
	Name     string     `json:"name"`
	State    AlertState `json:"state"`
	Query    string     `json:"query"`
	Duration float64    `json:"duration"`

	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
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
	AlertOrderFieldName        AlertOrderField = "NAME"
	AlertOrderFieldState       AlertOrderField = "STATE"
	AlertOrderFieldEnvironment AlertOrderField = "ENVIRONMENT"
)

var AllAlertOrderField = []AlertOrderField{
	AlertOrderFieldName,
	AlertOrderFieldState,
	AlertOrderFieldEnvironment,
}

func (e AlertOrderField) IsValid() bool {
	switch e {
	case AlertOrderFieldName, AlertOrderFieldState, AlertOrderFieldEnvironment:
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

	Alarms    []*PrometheusAlarm `json:"alarms"`
	RuleGroup string             `json:"ruleGroup"`
}

type PrometheusAlarm struct {
	Action      string     `json:"action"`
	Consequence string     `json:"consequence"`
	Summary     string     `json:"summary"`
	State       AlertState `json:"state"`
	Since       time.Time  `json:"since"`
}

func (e PrometheusAlert) ID() ident.Ident {
	return newIdent(AlertTypePrometheus, e.TeamSlug, e.EnvironmentName, e.Name)
}

func (PrometheusAlert) IsNode() {}

func (PrometheusAlert) IsAlert() {}

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

type AlertType int

const (
	AlertTypePrometheus AlertType = iota
	AlertTypeGrafana
)

func (t AlertType) String() string {
	switch t {
	case AlertTypePrometheus:
		return "Prometheus"
	case AlertTypeGrafana:
		return "Grafana"
	default:
		return "Unknown"
	}
}

// AlertTypeFromString returns the AlertType for the given string. If the string does not match any known type, -1 is returned.
func AlertTypeFromString(s string) (AlertType, error) {
	switch s {
	case "Prometheus":
		return AlertTypePrometheus, nil
	case "Grafana":
		return AlertTypeGrafana, nil
	default:
		return -1, fmt.Errorf("unknown workload type: %s", s)
	}
}
