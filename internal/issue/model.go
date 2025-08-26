package issue

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
)

type Issue interface {
	model.Node
}

type AivenIssue struct {
	ID           ident.Ident `json:"id"`
	ResourceName string      `json:"resourceName"`
	ResourceType string      `json:"resourceType"`
	Environment  string      `json:"environment"`
	Team         string      `json:"team"`
	Severity     Severity    `json:"severity"`
	Message      string      `json:"message"`
}

func (AivenIssue) IsIssue() {}

func (AivenIssue) IsNode() {}

type SQLInstanceIssue struct {
	ID           ident.Ident           `json:"id"`
	ResourceName string                `json:"resourceName"`
	ResourceType string                `json:"resourceType"`
	Environment  string                `json:"environment"`
	Team         string                `json:"team"`
	Severity     Severity              `json:"severity"`
	State        SQLInstanceIssueState `json:"state"`
	Message      string                `json:"message"`
}

func (SQLInstanceIssue) IsIssue() {}

func (SQLInstanceIssue) IsNode() {}

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityTodo     Severity = "TODO"
)

var AllSeverity = []Severity{
	SeverityCritical,
	SeverityWarning,
	SeverityTodo,
}

func (e Severity) IsValid() bool {
	switch e {
	case SeverityCritical, SeverityWarning, SeverityTodo:
		return true
	}
	return false
}

func (e Severity) String() string {
	return string(e)
}

func (e *Severity) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = Severity(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid Severity", str)
	}
	return nil
}

func (e Severity) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type SQLInstanceIssueState string

const (
	SQLInstanceIssueStateStopped                     SQLInstanceIssueState = "STOPPED"
	SQLInstanceIssueStateSQLInstanceStateUnspecified SQLInstanceIssueState = "SQL_INSTANCE_STATE_UNSPECIFIED"
	SQLInstanceIssueStateRunnable                    SQLInstanceIssueState = "RUNNABLE"
	SQLInstanceIssueStateSuspended                   SQLInstanceIssueState = "SUSPENDED"
	SQLInstanceIssueStatePendingDelete               SQLInstanceIssueState = "PENDING_DELETE"
	SQLInstanceIssueStatePendingCreate               SQLInstanceIssueState = "PENDING_CREATE"
	SQLInstanceIssueStateMaintenance                 SQLInstanceIssueState = "MAINTENANCE"
	SQLInstanceIssueStateFailed                      SQLInstanceIssueState = "FAILED"
	SQLInstanceIssueStateOnlineMaintenance           SQLInstanceIssueState = "ONLINE_MAINTENANCE"
	SQLInstanceIssueStateRepairing                   SQLInstanceIssueState = "REPAIRING"
)

var AllSQLInstanceIssueState = []SQLInstanceIssueState{
	SQLInstanceIssueStateStopped,
	SQLInstanceIssueStateSQLInstanceStateUnspecified,
	SQLInstanceIssueStateRunnable,
	SQLInstanceIssueStateSuspended,
	SQLInstanceIssueStatePendingDelete,
	SQLInstanceIssueStatePendingCreate,
	SQLInstanceIssueStateMaintenance,
	SQLInstanceIssueStateFailed,
	SQLInstanceIssueStateOnlineMaintenance,
	SQLInstanceIssueStateRepairing,
}

func (e SQLInstanceIssueState) IsValid() bool {
	switch e {
	case SQLInstanceIssueStateStopped, SQLInstanceIssueStateSQLInstanceStateUnspecified, SQLInstanceIssueStateRunnable, SQLInstanceIssueStateSuspended, SQLInstanceIssueStatePendingDelete, SQLInstanceIssueStatePendingCreate, SQLInstanceIssueStateMaintenance, SQLInstanceIssueStateFailed, SQLInstanceIssueStateOnlineMaintenance, SQLInstanceIssueStateRepairing:
		return true
	}
	return false
}

func (e SQLInstanceIssueState) String() string {
	return string(e)
}

func (e *SQLInstanceIssueState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = SQLInstanceIssueState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid SQLInstanceIssueState", str)
	}
	return nil
}

func (e SQLInstanceIssueState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type DeprecatedIngressIssue struct {
	ID           ident.Ident `json:"id"`
	ResourceName string      `json:"resourceName"`
	ResourceType string      `json:"resourceType"`
	Environment  string      `json:"environment"`
	Team         string      `json:"team"`
	Severity     Severity    `json:"severity"`
	Ingresses    []string    `json:"ingresses"`
}

func (DeprecatedIngressIssue) IsIssue() {}

func (DeprecatedIngressIssue) IsNode() {}
