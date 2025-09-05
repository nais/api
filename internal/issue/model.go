package issue

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/slug"
)

type Issue interface {
	model.Node
	IsIssue()
}

type Base struct {
	ID           ident.Ident  `json:"id"`
	Environment  string       `json:"environment"`
	Team         slug.Slug    `json:"team"`
	Severity     Severity     `json:"severity"`
	Message      string       `json:"message"`
	ResourceName string       `json:"-"`
	ResourceType ResourceType `json:"-"`
}

type OpenSearchIssue struct {
	Base
	Event string `json:"event"`
}

func (OpenSearchIssue) IsIssue() {}

func (OpenSearchIssue) IsNode() {}

type ValkeyIssue struct {
	Base
	Event string `json:"event"`
}

func (ValkeyIssue) IsIssue() {}

func (ValkeyIssue) IsNode() {}

type SqlInstanceVersionIssue struct {
	Base
}

func (SqlInstanceVersionIssue) IsIssue() {}

func (SqlInstanceVersionIssue) IsNode() {}

type SqlInstanceStateIssue struct {
	Base
	State sqlinstance.SQLInstanceState `json:"state"`
}

func (SqlInstanceStateIssue) IsIssue() {}

func (SqlInstanceStateIssue) IsNode() {}

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

type DeprecatedIngressIssue struct {
	Base
	Ingresses []string `json:"ingresses"`
}

func (DeprecatedIngressIssue) IsIssue() {}

func (DeprecatedIngressIssue) IsNode() {}

type ResourceType string

const (
	ResourceTypeOpensearch  ResourceType = "OPENSEARCH"
	ResourceTypeValkey      ResourceType = "VALKEY"
	ResourceTypeSQLInstance ResourceType = "SQLINSTANCE"
	ResourceTypeApplication ResourceType = "APPLICATION"
)

var AllResourceType = []ResourceType{
	ResourceTypeOpensearch,
	ResourceTypeValkey,
	ResourceTypeSQLInstance,
	ResourceTypeApplication,
}

func (e ResourceType) IsValid() bool {
	switch e {
	case ResourceTypeOpensearch, ResourceTypeValkey, ResourceTypeSQLInstance, ResourceTypeApplication:
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

type AivenIssueDetails struct {
	Event string `json:"event"`
}

type SQLInstanceIssueDetails struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

type DeprecatedIngressIssueDetails struct {
	Ingresses []string `json:"ingresses"`
}

type IssueType string

const (
	IssueTypeOpenSearch         IssueType = "OPENSEARCH"
	IssueTypeValkey             IssueType = "VALKEY"
	IssueTypeSqlInstanceState   IssueType = "SQLINSTANCE_STATE"
	IssueTypeSqlInstanceVersion IssueType = "SQLINSTANCE_VERSION"
	IssueTypeDeprecatedIngress  IssueType = "DEPRECATED_INGRESS"
)

var AllIssueType = []IssueType{
	IssueTypeOpenSearch,
	IssueTypeValkey,
	IssueTypeSqlInstanceState,
	IssueTypeSqlInstanceState,
	IssueTypeDeprecatedIngress,
}

func (e IssueType) IsValid() bool {
	switch e {
	case IssueTypeOpenSearch, IssueTypeValkey, IssueTypeSqlInstanceState, IssueTypeSqlInstanceVersion, IssueTypeDeprecatedIngress:
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

type IssueFilter struct {
	// Filter by resource type.
	ResourceType *ResourceType `json:"resourceType,omitempty"`
	// Filter by environment.
	Environments []string `json:"environments,omitempty"`
	// Filter by severity.
	Severity *Severity `json:"severity,omitempty"`
	// Filter by issue type.
	IssueType *IssueType `json:"issueType,omitempty"`
}

type (
	IssueConnection = pagination.Connection[Issue]
	IssueEdge       = pagination.Edge[Issue]
)

type IssueOrder struct {
	// Order by this field.
	Field IssueOrderField `json:"field"`
	// Order direction.
	Direction model.OrderDirection `json:"direction"`
}

func (o *IssueOrder) String() string {
	if o == nil {
		return ""
	}

	return strings.ToLower(o.Field.String() + ":" + o.Direction.String())
}

type IssueOrderField string

const (
	// Order by resource name.
	IssueOrderFieldResourceName IssueOrderField = "RESOURCE_NAME"
	// Order by severity.
	IssueOrderFieldSeverity IssueOrderField = "SEVERITY"
	// Order by environment.
	IssueOrderFieldEnvironment IssueOrderField = "ENVIRONMENT"
	// Order by resource type.
	IssueOrderFieldResourceType IssueOrderField = "RESOURCE_TYPE"
	// Order by issue type.
	IssueOrderFieldIssueType IssueOrderField = "ISSUE_TYPE"
)

var AllIssueOrderField = []IssueOrderField{
	IssueOrderFieldResourceName,
	IssueOrderFieldSeverity,
	IssueOrderFieldEnvironment,
	IssueOrderFieldResourceType,
	IssueOrderFieldIssueType,
}

func (e IssueOrderField) IsValid() bool {
	switch e {
	case IssueOrderFieldResourceName, IssueOrderFieldSeverity, IssueOrderFieldEnvironment, IssueOrderFieldResourceType, IssueOrderFieldIssueType:
		return true
	}
	return false
}

func (e IssueOrderField) String() string {
	return string(e)
}

func (e *IssueOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = IssueOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid IssueOrderField", str)
	}
	return nil
}

func (e IssueOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
