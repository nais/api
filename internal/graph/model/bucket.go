package model

import (
	"fmt"

	storage_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/storage/v1beta1"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Bucket struct {
	ID              scalar.Ident  `json:"id"`
	Env             Env           `json:"env"`
	Name            string        `json:"name"`
	ProjectID       string        `json:"projectId"`
	CascadingDelete bool          `json:"cascadingDelete"`
	GQLVars         BucketGQLVars `json:"-"`

	// TODO: check this via google api
	// PublicAccessPrevention   bool   `json:"publicAccessPrevention"`
	// RetentionPeriodDays      int    `json:"retentionPeriodDays"`
	// UniformBucketLevelAccess bool   `json:"uniformBucketLevelAccess"`
}

type BucketGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (Bucket) IsPersistence()    {}
func (b Bucket) GetName() string { return b.Name }

func ToBucket(u *unstructured.Unstructured, env string) (*Bucket, error) {
	bucket := &storage_cnrm_cloud_google_com_v1beta1.StorageBucket{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, bucket); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	projectId := bucket.GetAnnotations()["cnrm.cloud.google.com/project-id"]
	if projectId == "" {
		return nil, fmt.Errorf("missing project ID annotation")
	}

	teamSlug := bucket.GetNamespace()

	return &Bucket{
		ID: scalar.BucketIdent("bucket_" + env + "_" + teamSlug + "_" + bucket.GetName()),
		Env: Env{
			Name: env,
			Team: bucket.GetNamespace(),
		},
		Name:            bucket.Name,
		ProjectID:       projectId,
		CascadingDelete: bucket.Annotations["cnrm.cloud.google.com/deletion-policy"] == "abandon",
		GQLVars: BucketGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(bucket.OwnerReferences),
		},
	}, nil
}
