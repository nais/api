package model

import (
	"fmt"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Redis struct {
	Name    string       `json:"name"`
	ID      scalar.Ident `json:"id"`
	Env     Env          `json:"env"`
	Status  RedisStatus  `json:"status"`
	GQLVars RedisGQLVars `json:"-"`
}

type RedisInstanceAccess struct {
	Role    string                     `json:"role"`
	GQLVars RedisInstanceAccessGQLVars `json:"-"`
}

type RedisGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

type RedisInstanceAccessGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
	Env            Env
}

// TODO: Needs better name
type AccessEntry struct {
	OwnerReference *v1.OwnerReference
	Role           string
}

// TODO: Needs better name
type Access struct {
	Workloads []AccessEntry
}

func (Redis) IsPersistence()        {}
func (Redis) IsSearchNode()         {}
func (r Redis) GetName() string     { return r.Name }
func (r Redis) GetID() scalar.Ident { return r.ID }

func ToRedis(u *unstructured.Unstructured, envName string) (*Redis, error) {
	redis := &aiven_io_v1alpha1.Redis{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, redis); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	teamSlug := redis.GetNamespace()

	env := Env{
		Name: envName,
		Team: teamSlug,
	}
	r := &Redis{
		ID:   scalar.RedisIdent("redis_" + envName + "_" + teamSlug + "_" + redis.GetName()),
		Name: redis.Name,
		Env:  env,
		Status: RedisStatus{
			Conditions: func(conditions []v1.Condition) []*Condition {
				ret := make([]*Condition, len(conditions))
				for i, c := range conditions {
					ret[i] = &Condition{
						Type:               c.Type,
						Status:             string(c.Status),
						LastTransitionTime: c.LastTransitionTime.Time,
						Reason:             c.Reason,
						Message:            c.Message,
					}
				}

				return ret
			}(redis.Status.Conditions),
			State: redis.Status.State,
		},
		GQLVars: RedisGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(redis.OwnerReferences),
		},
	}

	return r, nil
}
