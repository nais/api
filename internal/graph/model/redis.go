package model

import (
	"fmt"

	"github.com/nais/api/internal/slug"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nais/api/internal/graph/scalar"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Redis struct {
	Name    string                `json:"name"`
	Access  []RedisInstanceAccess `json:"access"`
	ID      scalar.Ident          `json:"id"`
	Env     Env                   `json:"env"`
	GQLVars RedisGQLVars          `json:"-"`
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

func (Redis) IsPersistence()    {}
func (r Redis) GetName() string { return r.Name }

func (r Redis) GetID() scalar.Ident { return r.ID }

func ToRedis(u *unstructured.Unstructured, access *Access, envName string) (*Redis, error) {
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
		Access: func(a *Access) []RedisInstanceAccess {
			ret := make([]RedisInstanceAccess, 0)
			for _, w := range a.Workloads {
				ret = append(ret, RedisInstanceAccess{
					Role: w.Role,
					GQLVars: RedisInstanceAccessGQLVars{
						TeamSlug:       slug.Slug(teamSlug),
						OwnerReference: w.OwnerReference,
						Env:            env,
					},
				})
			}
			return ret
		}(access),
		GQLVars: RedisGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(redis.OwnerReferences), // app that might have created the Redis instance initially,
		},
	}

	return r, nil
}
