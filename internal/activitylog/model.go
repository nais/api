package activitylog

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type (
	ActivityLogEntryResourceType string
	ActivityLogEntryAction       string
)

const (
	ActivityLogEntryActionAdded   ActivityLogEntryAction = "ADDED"
	ActivityLogEntryActionCreated ActivityLogEntryAction = "CREATED"
	ActivityLogEntryActionDeleted ActivityLogEntryAction = "DELETED"
	ActivityLogEntryActionRemoved ActivityLogEntryAction = "REMOVED"
	ActivityLogEntryActionUpdated ActivityLogEntryAction = "UPDATED"
)

type ActivityLogEntry interface {
	model.Node
	GetUUID() uuid.UUID
	ID() ident.Ident
}

type (
	ActivityLogEntryConnection = pagination.Connection[ActivityLogEntry]
	ActivityLogEntryEdge       = pagination.Edge[ActivityLogEntry]
)

type GenericActivityLogEntry struct {
	Actor           string                       `json:"actor"`
	CreatedAt       time.Time                    `json:"createdAt"`
	EnvironmentName *string                      `json:"environmentName,omitempty"`
	Message         string                       `json:"message"`
	ResourceType    ActivityLogEntryResourceType `json:"resourceType"`
	ResourceName    string                       `json:"resourceName"`
	TeamSlug        *slug.Slug                   `json:"teamSlug,omitempty"`
	Action          ActivityLogEntryAction       `json:"-"`
	UUID            uuid.UUID                    `json:"-"`
	Data            []byte                       `json:"-"`
}

func (GenericActivityLogEntry) IsNode() {}

func (a GenericActivityLogEntry) ID() ident.Ident {
	return newIdent(a.UUID)
}

func (a GenericActivityLogEntry) GetUUID() uuid.UUID {
	return a.UUID
}

func (a GenericActivityLogEntry) WithMessage(message string) GenericActivityLogEntry {
	a.Message = message
	return a
}

// UnmarshalData unmarshals activity log entry data. Its inverse is [MarshalData].
func UnmarshalData[T any](entry GenericActivityLogEntry) (*T, error) {
	var data T
	if err := json.Unmarshal(entry.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshaling activity log entry data: %w", err)
	}

	return &data, nil
}

// TransformData unmarshals activity log entry data and calls the provided transformer function with the data as argument.
func TransformData[T any](entry GenericActivityLogEntry, f func(*T) *T) (*T, error) {
	data, err := UnmarshalData[T](entry)
	if err != nil {
		return nil, err
	}

	return f(data), nil
}

type ActivityLogFilter struct {
	ActivityTypes []ActivityLogActivityType `json:"activityTypes,omitempty"`
}

type ActivityLogActivityType string

func (e ActivityLogActivityType) IsValid() bool {
	_, ok := knownFilters[e]
	return ok
}

func (e ActivityLogActivityType) String() string {
	return string(e)
}

func (e *ActivityLogActivityType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ActivityLogActivityType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ActivityLogActivityType", str)
	}
	return nil
}

func (e ActivityLogActivityType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ActivityLogger interface {
	IsActivityLogger()
}
