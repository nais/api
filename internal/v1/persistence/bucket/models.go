package bucket

import (
	"fmt"
	"io"
	"strconv"

	storage_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/storage/v1beta1"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

type (
	BucketConnection     = pagination.Connection[*Bucket]
	BucketEdge           = pagination.Edge[*Bucket]
	BucketCorsConnection = pagination.Connection[*BucketCors]
	BucketCorsEdge       = pagination.Edge[*BucketCors]
)

type Bucket struct {
	Name                     string              `json:"name"`
	CascadingDelete          bool                `json:"cascadingDelete"`
	PublicAccessPrevention   string              `json:"publicAccessPrevention"`
	UniformBucketLevelAccess bool                `json:"uniformBucketLevelAccess"`
	Status                   *BucketStatus       `json:"status"`
	Cors                     []*BucketCors       `json:"-"`
	TeamSlug                 slug.Slug           `json:"-"`
	EnvironmentName          string              `json:"-"`
	WorkloadReference        *workload.Reference `json:"-"`
	ProjectID                string              `json:"-"`
}

func (Bucket) IsPersistence()     {}
func (Bucket) IsSearchNode()      {}
func (Bucket) IsNode()            {}
func (b *Bucket) GetName() string { return b.Name }

func (b *Bucket) GetNamespace() string { return b.TeamSlug.String() }

func (b *Bucket) GetLabels() map[string]string { return nil }

func (b *Bucket) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (b *Bucket) DeepCopyObject() runtime.Object {
	return b
}

func (b Bucket) ID() ident.Ident {
	return newIdent(b.TeamSlug, b.EnvironmentName, b.Name)
}

type BucketCors struct {
	MaxAgeSeconds   *int64   `json:"maxAgeSeconds,omitempty"`
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

func toBucketCors(cors []storage_cnrm_cloud_google_com_v1beta1.BucketCors) []*BucketCors {
	ret := make([]*BucketCors, len(cors))
	for i, c := range cors {
		ret[i] = &BucketCors{
			MaxAgeSeconds:   c.MaxAgeSeconds,
			Origins:         c.Origin,
			Methods:         c.Method,
			ResponseHeaders: c.ResponseHeader,
		}
	}
	return ret
}

func toBucketStatus(status storage_cnrm_cloud_google_com_v1beta1.StorageBucketStatus) *BucketStatus {
	// TODO: Implement
	return &BucketStatus{}
}

func toBucket(u *unstructured.Unstructured, env string) (*Bucket, error) {
	obj := &storage_cnrm_cloud_google_com_v1beta1.StorageBucket{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	projectId := obj.GetAnnotations()["cnrm.cloud.google.com/project-id"]
	if projectId == "" {
		return nil, fmt.Errorf("missing project ID annotation")
	}

	return &Bucket{
		Name:                     obj.Name,
		CascadingDelete:          obj.Annotations["cnrm.cloud.google.com/deletion-policy"] == "abandon",
		PublicAccessPrevention:   ptr.Deref(obj.Spec.PublicAccessPrevention, ""),
		WorkloadReference:        workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
		TeamSlug:                 slug.Slug(obj.GetNamespace()),
		EnvironmentName:          env,
		ProjectID:                projectId,
		UniformBucketLevelAccess: ptr.Deref(obj.Spec.UniformBucketLevelAccess, false),
		Cors:                     toBucketCors(obj.Spec.Cors),
		Status:                   toBucketStatus(obj.Status),
	}, nil
}
