package model

import (
	"fmt"

	"k8s.io/utils/ptr"

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

	PublicAccessPrevention   string `json:"publicAccessPrevention"`
	RetentionPeriodDays      int    `json:"retentionPeriodDays"`
	UniformBucketLevelAccess bool   `json:"uniformBucketLevelAccess"`
	Cors                     string `json:"cors"`
	SelfLink                 string `json:"selfLink"`
}

type BucketGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (Bucket) IsPersistence()    {}
func (b Bucket) GetName() string { return b.Name }

func (b Bucket) GetID() scalar.Ident { return b.ID }
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
		Name:                     bucket.Name,
		ProjectID:                projectId,
		CascadingDelete:          bucket.Annotations["cnrm.cloud.google.com/deletion-policy"] == "abandon",
		PublicAccessPrevention:   ptr.Deref(bucket.Spec.PublicAccessPrevention, ""),
		UniformBucketLevelAccess: ptr.Deref(bucket.Spec.UniformBucketLevelAccess, false),
		Cors: func(cors []storage_cnrm_cloud_google_com_v1beta1.BucketCors) string {
			ret := ""
			for _, c := range cors {
				for _, origin := range c.Origin {
					ret += origin
					for _, method := range c.Method {
						ret += " - " + method
					}
				}
			}
			return ret
		}(bucket.Spec.Cors),
		SelfLink: ptr.Deref(bucket.Status.SelfLink, ""),
		GQLVars: BucketGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(bucket.OwnerReferences),
		},
	}, nil
}
