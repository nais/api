package issuechecker

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
	ID           ident.Ident `json:"id"`
	ResourceName string      `json:"resourceName"`
	ResourceType string      `json:"resourceType"`
	Environment  string      `json:"environment"`
	Team         string      `json:"team"`
	Severity     Severity    `json:"severity"`
	Message      string      `json:"message"`
}

func (SQLInstanceIssue) IsIssue() {}

func (SQLInstanceIssue) IsNode() {}

type IssueType string

const (
	IssueTypeAivenAlert IssueType = "AIVEN_ALERT"
	IssueTypeCloudSQL   IssueType = "CLOUD_SQL"
)

var AllIssueType = []IssueType{
	IssueTypeAivenAlert,
	IssueTypeCloudSQL,
}

func (e IssueType) IsValid() bool {
	switch e {
	case IssueTypeAivenAlert, IssueTypeCloudSQL:
		return true
	}
	return false
}

func (e IssueType) String() string {
	return string(e)
}

func (e *IssueType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = IssueType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid IssueType", str)
	}
	return nil
}

func (e IssueType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

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
