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
	Name    string            `json:"name"`
	Access  string            `json:"access"`
	ID      scalar.Ident      `json:"id"`
	Env     Env               `json:"env"`
	GQLVars OpenSearchGQLVars `json:"-"`
}

type OpenSearchGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (OpenSearch) IsPersistence()        {}
func (o OpenSearch) GetName() string     { return o.Name }
func (o OpenSearch) GetID() scalar.Ident { return o.ID }

func ToOpenSearch(u *unstructured.Unstructured, env string) (*OpenSearch, error) {
	opensearch := &aiven_io_v1alpha1.Redis{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, opensearch); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	teamSlug := opensearch.GetNamespace()

	return &OpenSearch{
		ID:   scalar.OpenSearchIdent("opensearch_" + env + "_" + teamSlug + "_" + opensearch.GetName()),
		Name: opensearch.Name,
		Env: Env{
			Name: env,
			Team: teamSlug,
		},
		GQLVars: OpenSearchGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(opensearch.OwnerReferences),
		},
	}, nil
}
