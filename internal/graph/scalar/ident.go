package scalar

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/types"
)

type IdentType string

const (
	IdentTypeAnalysisTrail        IdentType = "analysisTrail"
	IdentTypeApp                  IdentType = "app"
	IdentTypeAuditEvent           IdentType = "auditEvent"
	IdentTypeAuditLog             IdentType = "auditLog"
	IdentTypeCorrelationID        IdentType = "correlationID"
	IdentTypeDeployKey            IdentType = "deployKey"
	IdentTypeDeployment           IdentType = "deployment"
	IdentTypeDeploymentResource   IdentType = "deploymentResource"
	IdentTypeDeploymentStatus     IdentType = "deploymentStatus"
	IdentTypeEnv                  IdentType = "env"
	IdentTypeFinding              IdentType = "finding"
	IdentTypeGitHubRepo           IdentType = "githubRepo"
	IdentTypeImage                IdentType = "image"
	IdentTypeJob                  IdentType = "job"
	IdentTypePod                  IdentType = "pod"
	IdentTypeReconcilerError      IdentType = "reconcilerError"
	IdentTypeSecret               IdentType = "secret"
	IdentTypeTeam                 IdentType = "team"
	IdentTypeUser                 IdentType = "user"
	IdentTypeUsersyncRun          IdentType = "usersyncRun"
	IdentTypeVulnerabilities      IdentType = "vulnerabilities"
	IdentTypeVulnerabilitySummary IdentType = "vulnerabilitySummary"
	IdentTypeWorkload             IdentType = "workload"

	idSeparator = "-"
)

type Ident struct {
	ID   string
	Type IdentType
}

func (i Ident) AsUUID() (uuid.UUID, error) {
	return uuid.Parse(i.ID)
}

func (i Ident) MarshalGQLContext(_ context.Context, w io.Writer) error {
	if i.ID == "" || i.Type == "" {
		return fmt.Errorf("id and type must be set")
	}
	v := url.Values{}
	v.Set("id", i.ID)
	v.Set("type", string(i.Type))
	_, err := w.Write([]byte(strconv.Quote(base64.URLEncoding.EncodeToString([]byte(v.Encode())))))
	return err
}

func (i *Ident) UnmarshalGQLContext(_ context.Context, v interface{}) error {
	ident, ok := v.(string)
	if !ok {
		return fmt.Errorf("ident must be a string")
	}

	bytes, err := base64.URLEncoding.DecodeString(ident)
	if err != nil {
		return err
	}

	values, err := url.ParseQuery(string(bytes))
	if err != nil {
		return err
	}

	i.ID = values.Get("id")
	i.Type = IdentType(values.Get("type"))

	return nil
}

func AppIdent(envName string, teamSlug slug.Slug, appName string) Ident {
	return newIdent(IdentTypeApp, envName, string(teamSlug), appName)
}

func DeployKeyIdent(teamSlug slug.Slug) Ident {
	return newIdent(IdentTypeDeployKey, string(teamSlug))
}

func EnvIdent(envName string) Ident {
	return newIdent(IdentTypeEnv, envName)
}

func JobIdent(jobName string) Ident {
	return newIdent(IdentTypeJob, jobName)
}

func PodIdent(id types.UID) Ident {
	return newIdent(IdentTypePod, string(id))
}

func TeamIdent(teamSlug slug.Slug) Ident {
	return newIdent(IdentTypeTeam, string(teamSlug))
}

func DeploymentIdent(id string) Ident {
	return newIdent(IdentTypeDeployment, id)
}

func DeploymentResourceIdent(id string) Ident {
	return newIdent(IdentTypeDeploymentResource, id)
}

func DeploymentStatusIdent(id string) Ident {
	return newIdent(IdentTypeDeploymentStatus, id)
}

func VulnerabilitiesIdent(id string) Ident {
	return newIdent(IdentTypeVulnerabilities, id)
}

func SecretIdent(envName string, teamSlug slug.Slug, secretName string) Ident {
	return newIdent(IdentTypeSecret, envName, string(teamSlug), secretName)
}

func AuditLogIdent(id uuid.UUID) Ident {
	return newIdent(IdentTypeAuditLog, id.String())
}

func AuditEventIdent(id uuid.UUID) Ident {
	return newIdent(IdentTypeAuditEvent, id.String())
}

func CorrelationID(id uuid.UUID) Ident {
	return newIdent(IdentTypeCorrelationID, id.String())
}

func UserIdent(userID uuid.UUID) Ident {
	return newIdent(IdentTypeUser, userID.String())
}

func UsersyncRunIdent(id uuid.UUID) Ident {
	return newIdent(IdentTypeUsersyncRun, id.String())
}

func GitHubRepository(repoName string) Ident {
	return newIdent(IdentTypeGitHubRepo, repoName)
}

func FindingIdent(id string) Ident {
	return newIdent(IdentTypeFinding, id)
}

func ImageIdent(name, version string) Ident {
	return newIdent(IdentTypeImage, name, version)
}

func WorkloadIdent(id string) Ident {
	return newIdent(IdentTypeWorkload, id)
}

func ReconcilerErrorIdent(id int) Ident {
	return newIdent(IdentTypeReconcilerError, strconv.Itoa(id))
}

func AnalysisTrailIdent(projectID, componentID, vulnerabilityID string) Ident {
	return newIdent(IdentTypeAnalysisTrail, projectID, componentID, vulnerabilityID)
}

func ImageVulnerabilitySummaryIdent(id string) Ident {
	return newIdent(IdentTypeVulnerabilitySummary, id)
}

func newIdent(t IdentType, id ...string) Ident {
	return Ident{
		ID:   strings.Join(id, idSeparator),
		Type: t,
	}
}
