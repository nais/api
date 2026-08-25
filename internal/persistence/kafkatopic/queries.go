package kafkatopic

import (
	"context"
	"strings"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/aivencredentials"
	"github.com/nais/api/internal/slug"
)

const maxTTLKafka = 365 * 24 * time.Hour // 365 days — used by Kafka

func GetByIdent(ctx context.Context, id ident.Ident) (*KafkaTopic, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*KafkaTopic, error) {
	return fromContext(ctx).watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *KafkaTopicOrder, filter *KafkaTopicFilter) (*KafkaTopicConnection, error) {
	all := ListAllForTeam(ctx, teamSlug, filter)

	if orderBy == nil {
		orderBy = &KafkaTopicOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	return SortFilterTopic.PaginatedList(ctx, all, page, orderBy.Field, orderBy.Direction, filter), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug, filter *KafkaTopicFilter) []*KafkaTopic {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, workloadName, poolName string, orderBy *KafkaTopicACLOrder) (*KafkaTopicACLConnection, error) {
	topics := fromContext(ctx).watcher.All()
	ret := make([]*KafkaTopicACL, 0)
	for _, t := range watcher.Objects(topics) {
		if t.Pool != poolName {
			continue
		}

		for _, acl := range t.ACLs {
			if stringMatch(teamSlug.String(), acl.TeamName) && stringMatch(workloadName, acl.WorkloadName) {
				ret = append(ret, acl)
			}
		}
	}
	orderTopicACLs(ctx, ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func CreateKafkaCredentials(ctx context.Context, input CreateKafkaCredentialsInput) (*CreateKafkaCredentialsPayload, error) {
	result, err := aivencredentials.CreateCredentials(ctx, ActivityLogEntryResourceTypeKafkaTopic, aivencredentials.CredentialRequest{
		TeamSlug:        input.TeamSlug,
		EnvironmentName: input.EnvironmentName,
		TTL:             input.TTL,
		MaxTTL:          maxTTLKafka,
		BuildSpec: func(namespace, secretName string, expiresAt time.Time) map[string]any {
			return map[string]any{
				"protected": true,
				"expiresAt": expiresAt.Format(time.RFC3339),
				"kafka": map[string]any{
					"pool":       "nav-" + input.EnvironmentName, // @TODO(chredvar): this will only work for Nav
					"secretName": secretName,
				},
			}
		},
		ExtractCreds: func(data map[string]string) any {
			return &KafkaCredentials{
				Username:       data["KAFKA_SCHEMA_REGISTRY_USER"],
				AccessCert:     data["KAFKA_CERTIFICATE"],
				AccessKey:      data["KAFKA_PRIVATE_KEY"],
				CaCert:         data["KAFKA_CA"],
				Brokers:        data["KAFKA_BROKERS"],
				SchemaRegistry: data["KAFKA_SCHEMA_REGISTRY"],
			}
		},
	})
	if err != nil {
		return nil, err
	}

	if err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionCredentialsCreated,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeKafkaTopic,
		ResourceName:    input.EnvironmentName,
		EnvironmentName: &input.EnvironmentName,
		TeamSlug:        &input.TeamSlug,
		Data: KafkaCredentialsCreatedActivityLogEntryData{
			TTL: input.TTL,
		},
	}); err != nil {
		fromContext(ctx).log.WithError(err).Warn("failed to create activity log entry for kafka credentials creation")
	}

	return &CreateKafkaCredentialsPayload{Credentials: result.(*KafkaCredentials)}, nil
}

func stringMatch(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	pattern = strings.Replace(pattern, "*", "", 1)
	return strings.HasPrefix(s, pattern)
}

func orderTopicACLs(ctx context.Context, acls []*KafkaTopicACL, orderBy *KafkaTopicACLOrder) {
	if orderBy == nil {
		orderBy = &KafkaTopicACLOrder{
			Field:     "TOPIC_NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	SortFilterTopicACL.Sort(ctx, acls, orderBy.Field, orderBy.Direction)
}
