package bigquery

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/persistence"
	"github.com/nais/api/internal/slug"
	bigquery_nais_io_v1 "github.com/nais/liberator/pkg/apis/google.nais.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

type (
	BigQueryDatasetConnection = pagination.Connection[*BigQueryDataset]
	BigQueryDatasetEdge       = pagination.Edge[*BigQueryDataset]
)

type BigQueryDataset struct {
	CascadingDelete bool                     `json:"cascadingDelete"`
	Description     string                   `json:"description"`
	Name            string                   `json:"name"`
	Access          []*BigQueryDatasetAccess `json:"access"`
	Status          *BigQueryDatasetStatus   `json:"-"`
	Location        string                   `json:"location"`
	TeamSlug        slug.Slug                `json:"-"`
	EnvironmentName string                   `json:"-"`
	OwnerReference  *metav1.OwnerReference   `json:"-"`
	ProjectID       string                   `json:"-"`
}

func (BigQueryDataset) IsPersistence() {}

func (BigQueryDataset) IsNode() {}

func (b BigQueryDataset) ID() ident.Ident {
	return newIdent(b.TeamSlug, b.EnvironmentName, b.Name)
}

type BigQueryDatasetAccess struct {
	Role  string `json:"role"`
	Email string `json:"email"`
}

type BigQueryDatasetStatus struct {
	CreationTime     time.Time          `json:"creationTime"`
	LastModifiedTime *time.Time         `json:"lastModifiedTime,omitempty"`
	Conditions       []metav1.Condition `json:"-"`
}

func toBigQueryDataset(u *unstructured.Unstructured, env string) (*BigQueryDataset, error) {
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
		Access: func(as []bigquery_nais_io_v1.DatasetAccess) []*BigQueryDatasetAccess {
			ret := make([]*BigQueryDatasetAccess, len(as))
			for i, a := range as {
				ret[i] = &BigQueryDatasetAccess{
					Role:  a.Role,
					Email: a.UserByEmail,
				}
			}
			return ret
		}(bqs.Spec.Access),
		EnvironmentName: env,
		Status: &BigQueryDatasetStatus{
			CreationTime: time.Unix(int64(bqs.Status.CreationTime), 0),
			LastModifiedTime: func(ts int) *time.Time {
				if ts == 0 {
					return nil
				}
				return ptr.To(time.Unix(int64(ts), 0))
			}(bqs.Status.LastModifiedTime),
			Conditions: bqs.Status.Conditions,
		},
		TeamSlug:       slug.Slug(teamSlug),
		OwnerReference: persistence.OwnerReference(bqs.OwnerReferences),
		ProjectID:      bqs.Spec.Project,
	}, nil
}
