package model

import (
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/k8s/v1alpha1"

	"k8s.io/utils/ptr"

	storage_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/storage/v1beta1"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Bucket struct {
	ID                       scalar.Ident  `json:"id"`
	Env                      Env           `json:"env"`
	Name                     string        `json:"name"`
	ProjectID                string        `json:"projectId"`
	CascadingDelete          bool          `json:"cascadingDelete"`
	Status                   BucketStatus  `json:"status"`
	PublicAccessPrevention   string        `json:"publicAccessPrevention"`
	RetentionPeriodDays      int           `json:"retentionPeriodDays"`
	UniformBucketLevelAccess bool          `json:"uniformBucketLevelAccess"`
	Cors                     []BucketCors  `json:"cors"`
	GQLVars                  BucketGQLVars `json:"-"`
}

type BucketGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (Bucket) IsPersistence() {}
func (Bucket) IsSearchNode()  {}

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
		Cors: func(cors []storage_cnrm_cloud_google_com_v1beta1.BucketCors) []BucketCors {
			ret := make([]BucketCors, len(cors))
			for i, c := range cors {
				ret[i] = BucketCors{
					MaxAgeSeconds:   c.MaxAgeSeconds,
					Origins:         c.Origin,
					Methods:         c.Method,
					ResponseHeaders: c.ResponseHeader,
				}
			}
			return ret
		}(bucket.Spec.Cors),
		Status: BucketStatus{
			Conditions: func(conditions []v1alpha1.Condition) []*Condition {
				ret := make([]*Condition, len(conditions))
				for i, c := range conditions {
					t, err := time.Parse(time.RFC3339, c.LastTransitionTime)
					if err != nil {
						t = time.Unix(0, 0)
					}
					ret[i] = &Condition{
						Type:               c.Type,
						Status:             string(c.Status),
						LastTransitionTime: t,
						Reason:             c.Reason,
						Message:            c.Message,
					}
				}

				return ret
			}(bucket.Status.Conditions),
			SelfLink: ptr.Deref(bucket.Status.SelfLink, ""),
		},
		GQLVars: BucketGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(bucket.OwnerReferences),
		},
	}, nil
}
