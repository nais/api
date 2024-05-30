package model

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	bigquery_nais_io_v1 "github.com/nais/liberator/pkg/apis/google.nais.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

type BigQueryDataset struct {
	CascadingDelete bool                    `json:"cascadingDelete"` // TODO: These don't actually ever exist in the cluster??
	Description     string                  `json:"description"`
	Env             Env                     `json:"env"`
	GQLVars         BigQueryDatasetGQLVars  `json:"-"`
	ID              scalar.Ident            `json:"id"`
	Name            string                  `json:"name"`
	Access          []BigQueryDatasetAccess `json:"access"` // TODO: There's some incongruency with what we have in the cluster here.
	ProjectID       string                  `json:"projectId"`
	Location        string                  `json:"location"`
	Status          BigQueryDatasetStatus   `json:"status"`
}

type BigQueryDatasetGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (BigQueryDataset) IsPersistence() {}

func (in BigQueryDataset) GetName() string { return in.Name }

func (in BigQueryDataset) GetID() scalar.Ident { return in.ID }

func ToBigQueryDataset(u *unstructured.Unstructured, env string) (*BigQueryDataset, error) {
	bqs := &bigquery_nais_io_v1.BigQueryDataset{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, bqs); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	teamSlug := bqs.GetNamespace()

	return &BigQueryDataset{
		CascadingDelete: bqs.Spec.CascadingDelete,
		Description:     bqs.Spec.Description,
		Name:            bqs.GetName(),
		Location:        bqs.Spec.Location,
		Access: func(as []bigquery_nais_io_v1.DatasetAccess) []BigQueryDatasetAccess {
			ret := make([]BigQueryDatasetAccess, len(as))
			for i, a := range as {
				ret[i] = BigQueryDatasetAccess{
					Role:  a.Role,
					Email: a.UserByEmail,
				}
			}
			return ret
		}(bqs.Spec.Access),
		ID: scalar.BigQueryDatasetIdent("bigquerydataset_" + env + "_" + teamSlug + "_" + bqs.GetName()),
		Env: Env{
			Team: teamSlug,
			Name: env,
		},
		Status: BigQueryDatasetStatus{
			CreationTime: time.Unix(int64(bqs.Status.CreationTime), 0),
			LastModifiedTime: func(ts int) *time.Time {
				if ts == 0 {
					return nil
				}
				return ptr.To(time.Unix(int64(ts), 0))
			}(bqs.Status.LastModifiedTime),
			Conditions: func(conditions []v1.Condition) []*Condition {
				ret := make([]*Condition, len(conditions))
				for i, c := range conditions {
					t := c.LastTransitionTime.Time
					ret[i] = &Condition{
						Type:               c.Type,
						Status:             string(c.Status),
						LastTransitionTime: t,
						Reason:             c.Reason,
						Message:            c.Message,
					}
				}

				return ret
			}(bqs.Status.Conditions),
		},
		GQLVars: BigQueryDatasetGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(bqs.OwnerReferences),
		},
		ProjectID: bqs.Spec.Project,
	}, nil
}
