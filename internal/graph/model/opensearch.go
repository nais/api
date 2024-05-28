package model

import (
	"fmt"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type OpenSearch struct {
	// The opensearch instance name
	Name    string                     `json:"name"`
	Access  []OpenSearchInstanceAccess `json:"access"`
	ID      scalar.Ident               `json:"id"`
	Env     Env                        `json:"env"`
	Cost    string                     `json:"cost"`
	GQLVars OpenSearchGQLVars          `json:"-"`
}

type OpenSearchInstanceAccess struct {
	Role    string                          `json:"role"`
	GQLVars OpenSearchInstanceAccessGQLVars `json:"-"`
}

type OpenSearchGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

type OpenSearchInstanceAccessGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
	Env            Env
}

func (OpenSearch) IsPersistence()        {}
func (o OpenSearch) GetName() string     { return o.Name }
func (o OpenSearch) GetID() scalar.Ident { return o.ID }

func ToOpenSearch(u *unstructured.Unstructured, access *Access, envName string) (*OpenSearch, error) {
	openSearch := &aiven_io_v1alpha1.Redis{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, openSearch); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	teamSlug := openSearch.GetNamespace()
	env := Env{
		Name: envName,
		Team: teamSlug,
	}
	return &OpenSearch{
		ID:   scalar.OpenSearchIdent("opensearch_" + envName + "_" + teamSlug + "_" + openSearch.GetName()),
		Name: openSearch.Name,
		Env:  env,
		Access: func(a *Access) []OpenSearchInstanceAccess {
			ret := make([]OpenSearchInstanceAccess, 0)
			for _, w := range a.Workloads {
				ret = append(ret, OpenSearchInstanceAccess{
					Role: w.Role,
					GQLVars: OpenSearchInstanceAccessGQLVars{
						TeamSlug:       slug.Slug(teamSlug),
						OwnerReference: w.OwnerReference,
						Env:            env,
					},
				})
			}
			return ret
		}(access),
		GQLVars: OpenSearchGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(openSearch.OwnerReferences),
		},
	}, nil
}
