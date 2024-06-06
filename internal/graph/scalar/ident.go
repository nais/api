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
	IdentTypeApp                         IdentType = "app"
	IdentTypeAuditLog                    IdentType = "auditLog"
	IdentTypeBigQueryDataset             IdentType = "bigQueryDataset"
	IdentTypeBucket                      IdentType = "bucket"
	IdentTypeCorrelationID               IdentType = "correlationID"
	IdentTypeDeployKey                   IdentType = "deployKey"
	IdentTypeDeployment                  IdentType = "deployment"
	IdentTypeDeploymentResource          IdentType = "deploymentResource"
	IdentTypeDeploymentStatus            IdentType = "deploymentStatus"
	IdentTypeEnv                         IdentType = "env"
	IdentTypeGitHubRepo                  IdentType = "githubRepo"
	IdentTypeJob                         IdentType = "job"
	IdentTypeOpenSearch                  IdentType = "openSearch"
	IdentTypePod                         IdentType = "pod"
	IdentTypeRedis                       IdentType = "redis"
	IdentTypeSecret                      IdentType = "secret"
	IdentTypeSqlDatabase                 IdentType = "sqlDatabase"
	IdentTypeSqlInstance                 IdentType = "sqlInstance"
	IdentTypeTeam                        IdentType = "team"
	IdentTypeUser                        IdentType = "user"
	IdentTypeVulnerabilities             IdentType = "vulnerabilities"
	IdentTypeKafkaTopic                  IdentType = "kafkaTopic"
	IdentTypeFinding                     IdentType = "finding"
	IdentTypeImage                       IdentType = "image"
	IdentTypeDependencyTrackProjectIdent IdentType = "dependencyTrackProjectIdent"
	IdentTypeWorkload                    IdentType = "workload"
	IdentTypeAnalysisTrail               IdentType = "analysisTrail"
	IdentTypeVulnerabilitySummary        IdentType = "vulnerabilitySummary"
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
	return newIdent(id.String(), IdentTypeUser)
}

func GitHubRepository(name string) Ident {
	return newIdent(name, IdentTypeGitHubRepo)
}

func SqlInstanceIdent(id string) Ident {
	return newIdent(id, IdentTypeSqlInstance)
}

func SqlDatabaseIdent(id string) Ident {
	return newIdent(id, IdentTypeSqlDatabase)
}

func BucketIdent(id string) Ident {
	return newIdent(id, IdentTypeBucket)
}

func BigQueryDatasetIdent(id string) Ident {
	return newIdent(id, IdentTypeBigQueryDataset)
}

func RedisIdent(id string) Ident {
	return newIdent(id, IdentTypeRedis)
}

func KafkaTopicIdent(id string) Ident {
	return newIdent(id, IdentTypeKafkaTopic)
}

func OpenSearchIdent(envName string, teamSlug slug.Slug, instanceName string) Ident {
	return newIdent(envName+"-"+string(teamSlug)+"-"+instanceName, IdentTypeOpenSearch)
}

func FindingIdent(id string) Ident {
	return newIdent(id, IdentTypeFinding)
}

func ImageIdent(name, version string) Ident {
	return newIdent(fmt.Sprintf("%s-%s", name, version), IdentTypeImage)
}

func WorkloadIdent(id string) Ident {
	return newIdent(id, IdentTypeWorkload)
}

func AnalysisTrailIdent(projectID, componentID, vulnerabilityID string) Ident {
	return newIdent(fmt.Sprintf("%s-%s-%s", projectID, componentID, vulnerabilityID), IdentTypeAnalysisTrail)
}

func ImageVulnerabilitySummaryIdent(id string) Ident {
	return newIdent(id, IdentTypeVulnerabilitySummary)
}

func newIdent(id string, t IdentType) Ident {
	return Ident{
		ID:   id,
		Type: t,
	}
}
