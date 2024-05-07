package model

import (
	"fmt"

	storage_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/storage/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Bucket struct {
	CascadingDelete bool   `json:"cascadingDelete"`
	Name            string `json:"name"`
	//	 check this via google api
	//		PublicAccessPrevention   bool   `json:"publicAccessPrevention"`
	//		RetentionPeriodDays      int    `json:"retentionPeriodDays"`
	//		UniformBucketLevelAccess bool   `json:"uniformBucketLevelAccess"`
}

func (Bucket) IsPersistence()       {}
func (this Bucket) GetName() string { return this.Name }

func ToBucket(u *unstructured.Unstructured) (*Bucket, error) {
	bucket := &storage_cnrm_cloud_google_com_v1beta1.StorageBucket{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, bucket); err != nil {
		return nil, fmt.Errorf("converting to SQL database: %w", err)
	}
	return &Bucket{
		CascadingDelete: bucket.Annotations["cnrm.cloud.google.com/deletion-policy"] == "abandon",
		Name:            bucket.Name,
	}, nil

	// return &SQLDatabase{
	// 	Name:           sqlDatabase.Name,
	// 	Charset:        sqlDatabase.Spec.Charset,
	// 	Collation:      sqlDatabase.Spec.Collation,
	// 	DeletionPolicy: sqlDatabase.Spec.DeletionPolicy,
	// 	InstanceRef:    sqlDatabase.Spec.InstanceRef.Name,
	// 	Healthy:        IsHealthy(sqlDatabase.Status.Conditions),
	// 	Conditions: func() []*Condition {
	// 		ret := make([]*Condition, 0)
	// 		for _, condition := range sqlDatabase.Status.Conditions {
	// 			ret = append(ret, &Condition{
	// 				Type:               condition.Type,
	// 				Status:             string(condition.Status),
	// 				Reason:             condition.Reason,
	// 				Message:            condition.Message,
	// 				LastTransitionTime: condition.LastTransitionTime,
	// 			})
	// 		}
	// 		return ret
	// 	}(),
	// }, nil
}
