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
	Name    string            `json:"name"`
	ID      scalar.Ident      `json:"id"`
	Env     Env               `json:"env"`
	Status  OpenSearchStatus  `json:"status"`
	GQLVars OpenSearchGQLVars `json:"-"`
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

func (OpenSearch) IsPersistence() {}
func (OpenSearch) IsSearchNode()  {}

func ToOpenSearch(u *unstructured.Unstructured, envName string) (*OpenSearch, error) {
	openSearch := &aiven_io_v1alpha1.OpenSearch{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, openSearch); err != nil {
		return nil, fmt.Errorf("converting to OpenSearch: %w", err)
	}

	teamSlug := openSearch.GetNamespace()
	if teamSlug == "" {
		return nil, fmt.Errorf("missing namespace")
	}

	instanceName := openSearch.GetName()
	if instanceName == "" {
		return nil, fmt.Errorf("missing instance name")
	}

	return &OpenSearch{
		ID:   scalar.OpenSearchIdent(envName, slug.Slug(teamSlug), instanceName),
		Name: instanceName,
		Env: Env{
			Name: envName,
			Team: teamSlug,
		},
		Status: OpenSearchStatus{
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
			}(openSearch.Status.Conditions),
			State: openSearch.Status.State,
		},
		GQLVars: OpenSearchGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(openSearch.OwnerReferences),
		},
	}, nil
}
