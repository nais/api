package deployment

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/deployment/deploymentsql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/workload"
)

type (
	DeploymentConnection         = pagination.Connection[*Deployment]
	DeploymentEdge               = pagination.Edge[*Deployment]
	DeploymentStatusConnection   = pagination.Connection[*DeploymentStatus]
	DeploymentStatusEdge         = pagination.Edge[*DeploymentStatus]
	DeploymentResourceConnection = pagination.Connection[*DeploymentResource]
	DeploymentResourceEdge       = pagination.Edge[*DeploymentResource]
)

type Deployment struct {
	CreatedAt       time.Time `json:"createdAt"`
	Repository      *string   `json:"repository"`
	UUID            uuid.UUID `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}

func (Deployment) IsNode() {}

func (d *Deployment) ID() ident.Ident {
	return newDeploymentIdent(d.UUID)
}

type DeploymentResource struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"-"`
	Group     string    `json:"-"`
	Version   string    `json:"-"`
	Namespace string    `json:"-"`
	UUID      uuid.UUID `json:"-"`
}

func (DeploymentResource) IsNode() {}

func (d *DeploymentResource) ID() ident.Ident {
	return newDeploymentResourceIdent(d.UUID)
}

type DeploymentStatus struct {
	CreatedAt time.Time             `json:"createdAt"`
	State     DeploymentStatusState `json:"state"`
	Message   string                `json:"message,omitempty"`
	UUID      uuid.UUID             `json:"-"`
}

func (DeploymentStatus) IsNode() {}

func (d *DeploymentStatus) ID() ident.Ident {
	return newDeploymentStatusIdent(d.UUID)
}

type DeploymentStatusState string

const (
	DeploymentStatusStateSuccess    DeploymentStatusState = "SUCCESS"
	DeploymentStatusStateError      DeploymentStatusState = "ERROR"
	DeploymentStatusStateFailure    DeploymentStatusState = "FAILURE"
	DeploymentStatusStateInactive   DeploymentStatusState = "INACTIVE"
	DeploymentStatusStateInProgress DeploymentStatusState = "IN_PROGRESS"
	DeploymentStatusStateQueued     DeploymentStatusState = "QUEUED"
	DeploymentStatusStatePending    DeploymentStatusState = "PENDING"
)

var AllDeploymentStatusState = []DeploymentStatusState{
	DeploymentStatusStateSuccess,
	DeploymentStatusStateError,
	DeploymentStatusStateFailure,
	DeploymentStatusStateInactive,
	DeploymentStatusStateInProgress,
	DeploymentStatusStateQueued,
	DeploymentStatusStatePending,
}

func (e DeploymentStatusState) IsValid() bool {
	return slices.Contains(AllDeploymentStatusState, e)
}

func (e DeploymentStatusState) String() string {
	return string(e)
}

func (e *DeploymentStatusState) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = DeploymentStatusState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid DeploymentStatusState", str)
	}
	return nil
}

func (e DeploymentStatusState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type DeploymentInfo struct {
	Deployer  *string    `json:"deployer,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
	CommitSha *string    `json:"commitSha,omitempty"`
	URL       *string    `json:"url,omitempty"`

	TeamSlug        slug.Slug     `json:"-"`
	EnvironmentName string        `json:"-"`
	WorkloadName    string        `json:"-"`
	WorkloadType    workload.Type `json:"-"`
}

type ChangeDeploymentKeyInput struct {
	TeamSlug slug.Slug `json:"team"`
}

type ChangeDeploymentKeyPayload struct {
	DeploymentKey *DeploymentKey `json:"deploymentKey,omitempty"`
}

type DeploymentKey struct {
	Key      string    `json:"key"`
	Created  time.Time `json:"created"`
	Expires  time.Time `json:"expires"`
	TeamSlug slug.Slug `json:"-"`
}

func toGraphDeploymentKey(d *hookd.DeployKey, teamSlug slug.Slug) *DeploymentKey {
	return &DeploymentKey{
		Key:      d.Key,
		Created:  d.Created,
		Expires:  d.Expires,
		TeamSlug: teamSlug,
	}
}

func (d DeploymentKey) ID() ident.Ident {
	return newDeploymentKeyIdent(d.TeamSlug)
}

func (DeploymentKey) IsNode() {}

func toGraphDeployment(row *deploymentsql.Deployment) *Deployment {
	return &Deployment{
		CreatedAt:       row.CreatedAt.Time,
		Repository:      row.Repository,
		UUID:            row.ID,
		TeamSlug:        row.TeamSlug,
		EnvironmentName: row.EnvironmentName,
	}
}

func toGraphDeploymentResource(row *deploymentsql.DeploymentK8sResource) *DeploymentResource {
	return &DeploymentResource{
		Group:     row.Group,
		Version:   row.Version,
		Kind:      row.Kind,
		Name:      row.Name,
		Namespace: row.Namespace,
		UUID:      row.ID,
	}
}

func toGraphDeploymentStatus(row *deploymentsql.DeploymentStatus) *DeploymentStatus {
	var state DeploymentStatusState
	switch row.State {
	case deploymentsql.DeploymentStateSuccess:
		state = DeploymentStatusStateSuccess
	case deploymentsql.DeploymentStateError:
		state = DeploymentStatusStateError
	case deploymentsql.DeploymentStateFailure:
		state = DeploymentStatusStateFailure
	case deploymentsql.DeploymentStateInactive:
		state = DeploymentStatusStateInactive
	case deploymentsql.DeploymentStateInProgress:
		state = DeploymentStatusStateInProgress
	case deploymentsql.DeploymentStateQueued:
		state = DeploymentStatusStateQueued
	case deploymentsql.DeploymentStatePending:
		state = DeploymentStatusStatePending
	default:
		state = DeploymentStatusStatePending
	}
	return &DeploymentStatus{
		CreatedAt: row.CreatedAt.Time,
		State:     state,
		Message:   row.Message,
		UUID:      row.ID,
	}
}
