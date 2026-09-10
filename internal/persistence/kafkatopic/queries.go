package kafkatopic

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strconv"
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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func Update(ctx context.Context, input UpdateKafkaTopicInput) (*UpdateKafkaTopicPayload, error) {
	topic, err := Get(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
	if err != nil {
		return nil, err
	}

	patch := []map[string]any{}
	indicesToRevoke := make([]int, 0, len(input.RevokeGrants))
	revokedGrants := make([]KafkaTopicUpdatedActivityLogEntryDataGrant, 0, len(input.RevokeGrants))
	seen := make(map[string]struct{})
	for _, grant := range input.RevokeGrants {
		key := grant.Access.AivenAccess() + "|" + grant.TeamName + "|" + grant.Subject
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		aclIndex := slices.IndexFunc(topic.ACLs, func(acl *KafkaTopicACL) bool {
			return acl.Access == grant.Access && acl.TeamName == grant.TeamName && acl.WorkloadName == grant.Subject
		})
		if aclIndex != -1 {
			indicesToRevoke = append(indicesToRevoke, aclIndex)
			revokedGrants = append(revokedGrants, KafkaTopicUpdatedActivityLogEntryDataGrant{
				Subject:  grant.Subject,
				TeamName: grant.TeamName,
				Access:   grant.Access,
			})
		}
	}

	sort.Sort(sort.Reverse(sort.IntSlice(indicesToRevoke)))
	for _, aclIndex := range indicesToRevoke {
		patch = append(patch, map[string]any{
			"op":   "remove",
			"path": "/spec/acl/" + strconv.Itoa(aclIndex),
		})
	}

	seen = make(map[string]struct{})
	addedGrants := make([]KafkaTopicUpdatedActivityLogEntryDataGrant, 0, len(input.AddGrants))
	for _, grant := range input.AddGrants {
		key := grant.Access.AivenAccess() + "|" + grant.TeamName + "|" + grant.Subject
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		if slices.ContainsFunc(topic.ACLs, func(a *KafkaTopicACL) bool {
			return a.Access == grant.Access && a.TeamName == grant.TeamName && a.WorkloadName == grant.Subject
		}) {
			continue
		}

		patch = append(patch, map[string]any{
			"op":   "add",
			"path": "/spec/acl/-",
			"value": map[string]string{
				"team":        grant.TeamName,
				"application": grant.Subject,
				"access":      grant.Access.AivenAccess(),
			},
		})
		addedGrants = append(addedGrants, KafkaTopicUpdatedActivityLogEntryDataGrant{
			Subject:  grant.Subject,
			TeamName: grant.TeamName,
			Access:   grant.Access,
		})
	}

	if len(patch) == 0 {
		return &UpdateKafkaTopicPayload{KafkaTopic: topic}, nil
	}

	client, err := fromContext(ctx).watcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, input.TeamSlug.String())
	if err != nil {
		return nil, err
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	res, err := client.Patch(ctx, topic.Name, types.JSONPatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return nil, err
	}

	updatedTopic, err := toKafkaTopic(res, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	if err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionUpdated,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeKafkaTopic,
		ResourceName:    input.Name,
		TeamSlug:        &input.TeamSlug,
		EnvironmentName: &input.EnvironmentName,
		Data: KafkaTopicUpdatedActivityLogEntryData{
			AddedGrants:   addedGrants,
			RevokedGrants: revokedGrants,
		},
	}); err != nil {
		fromContext(ctx).log.WithError(err).Warn("failed to create activity log entry for kafka topic update")
	}

	return &UpdateKafkaTopicPayload{KafkaTopic: updatedTopic}, nil
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
