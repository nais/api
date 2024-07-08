package bucket

import (
	"fmt"
	storage_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/storage/v1beta1"
	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/persistence"
	"github.com/nais/api/internal/slug"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"strconv"
)

type (
	BucketConnection = pagination.Connection[*Bucket]
	BucketEdge       = pagination.Edge[*Bucket]
)

type Bucket struct {
	Name                     string                 `json:"name"`
	CascadingDelete          bool                   `json:"cascadingDelete"`
	PublicAccessPrevention   string                 `json:"publicAccessPrevention"`
	RetentionPeriodDays      int                    `json:"retentionPeriodDays"`
	UniformBucketLevelAccess bool                   `json:"uniformBucketLevelAccess"`
	Cors                     []*BucketCors          `json:"cors"`
	Status                   BucketStatus           `json:"-"`
	TeamSlug                 slug.Slug              `json:"-"`
	EnvironmentName          string                 `json:"-"`
	OwnerReference           *metav1.OwnerReference `json:"-"`
	ProjectID                string                 `json:"-"`
}

func (Bucket) IsPersistence() {}

func (Bucket) IsNode() {}

func (b Bucket) ID() ident.Ident {
	return newIdent(b.TeamSlug, b.EnvironmentName, b.Name)
}

type BucketCors struct {
	MaxAgeSeconds   *int     `json:"maxAgeSeconds,omitempty"`
	Methods         []string `json:"methods"`
	Origins         []string `json:"origins"`
	ResponseHeaders []string `json:"responseHeaders"`
}

type BucketOrder struct {
	Field     BucketOrderField       `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type BucketStatus struct {
	State string `json:"state"`
}

type BucketOrderField string

const (
	BucketOrderFieldName        BucketOrderField = "NAME"
	BucketOrderFieldEnvironment BucketOrderField = "ENVIRONMENT"
)

func (e BucketOrderField) IsValid() bool {
	switch e {
	case BucketOrderFieldName, BucketOrderFieldEnvironment:
		return true
	}
	return false
}

func (e BucketOrderField) String() string {
	return string(e)
}

func (e *BucketOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = BucketOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid BucketOrderField", str)
	}
	return nil
}

func (e BucketOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toBucket(u *unstructured.Unstructured, env string) (*Bucket, error) {
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
		OwnerReference:           persistence.OwnerReference(bucket.OwnerReferences),
		TeamSlug:                 slug.Slug(teamSlug),
		EnvironmentName:          env,
		Name:                     bucket.Name,
		ProjectID:                projectId,
		CascadingDelete:          bucket.Annotations["cnrm.cloud.google.com/deletion-policy"] == "abandon",
		PublicAccessPrevention:   ptr.Deref(bucket.Spec.PublicAccessPrevention, ""),
		UniformBucketLevelAccess: ptr.Deref(bucket.Spec.UniformBucketLevelAccess, false),

		/*
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

		*/
	}, nil
}
