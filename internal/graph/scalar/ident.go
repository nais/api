package scalar

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/types"
)

type IdentType string

const (
	IdentTypeApp                IdentType = "app"
	IdentTypeAuditLog           IdentType = "auditLog"
	IdentTypeCorrelationID      IdentType = "correlationID"
	IdentTypeDeployKey          IdentType = "deployKey"
	IdentTypeDeployment         IdentType = "deployment"
	IdentTypeDeploymentResource IdentType = "deploymentResource"
	IdentTypeDeploymentStatus   IdentType = "deploymentStatus"
	IdentTypeEnv                IdentType = "env"
	IdentTypeJob                IdentType = "job"
	IdentTypePod                IdentType = "pod"
	IdentTypeSecret             IdentType = "secret"
	IdentTypeTeam               IdentType = "team"
	IdentTypeVulnerabilities    IdentType = "vulnerabilities"
	IdentTypeGitHubRepo         IdentType = "githubRepo"
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

func AppIdent(id string) Ident {
	return newIdent(id, IdentTypeApp)
}

func DeployKeyIdent(id string) Ident {
	return newIdent(id, IdentTypeDeployKey)
}

func EnvIdent(id string) Ident {
	return newIdent(id, IdentTypeEnv)
}

func JobIdent(id string) Ident {
	return newIdent(id, IdentTypeJob)
}

func PodIdent(id types.UID) Ident {
	return newIdent(string(id), IdentTypePod)
}

func TeamIdent(id slug.Slug) Ident {
	return newIdent(id.String(), IdentTypeTeam)
}

func DeploymentIdent(id string) Ident {
	return newIdent(id, IdentTypeDeployment)
}

func DeploymentResourceIdent(id string) Ident {
	return newIdent(id, IdentTypeDeploymentResource)
}

func DeploymentStatusIdent(id string) Ident {
	return newIdent(id, IdentTypeDeploymentStatus)
}

func VulnerabilitiesIdent(id string) Ident {
	return newIdent(id, IdentTypeVulnerabilities)
}

func SecretIdent(id string) Ident {
	return newIdent(id, IdentTypeSecret)
}

func AuditLogIdent(id uuid.UUID) Ident {
	return newIdent(id.String(), IdentTypeAuditLog)
}

func CorrelationID(id uuid.UUID) Ident {
	return newIdent(id.String(), IdentTypeCorrelationID)
}

func UserIdent(id uuid.UUID) Ident {
	return newIdent(id.String(), "user")
}

func GitHubRepository(name string) Ident {
	return newIdent(name, IdentTypeGitHubRepo)
}

func newIdent(id string, t IdentType) Ident {
	return Ident{
		ID:   id,
		Type: t,
	}
}
