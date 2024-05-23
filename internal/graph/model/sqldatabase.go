package model

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/graph/scalar"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/k8s/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type SQLDatabase struct {
	Charset        *string      `json:"charset"`
	Collation      *string      `json:"collation"`
	DeletionPolicy *string      `json:"deletionPolicy"`
	InstanceRef    string       `json:"instanceRef"`
	Healthy        bool         `json:"healthy"`
	Name           string       `json:"name"`
	Conditions     []*Condition `json:"conditions"`
	ID             scalar.Ident `json:"id"`
}

func ToSqlDatabase(u *unstructured.Unstructured, sqlInstanceName, env string) (*SQLDatabase, error) {
	sqlDatabase := &sql_cnrm_cloud_google_com_v1beta1.SQLDatabase{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sqlDatabase); err != nil {
		return nil, fmt.Errorf("converting to SQL database: %w", err)
	}

	// TODO: does not handle abandoned databases
	if sqlDatabase.Spec.InstanceRef.Name != sqlInstanceName {
		return nil, nil
	}

	teamSlug := sqlDatabase.GetNamespace()
	return &SQLDatabase{
		ID:             scalar.SqlDatabaseIdent("sqldatabase_" + env + "_" + teamSlug + "_" + sqlDatabase.GetName()),
		Name:           sqlDatabase.Name,
		Charset:        sqlDatabase.Spec.Charset,
		Collation:      sqlDatabase.Spec.Collation,
		DeletionPolicy: sqlDatabase.Spec.DeletionPolicy,
		InstanceRef:    sqlDatabase.Spec.InstanceRef.Name,
		Healthy:        IsHealthy(sqlDatabase.Status.Conditions),
		Conditions: func() []*Condition {
			ret := make([]*Condition, 0)
			for _, condition := range sqlDatabase.Status.Conditions {
				t, err := time.Parse(time.RFC3339, condition.LastTransitionTime)
				if err != nil {
					t = time.Unix(0, 0)
				}
				ret = append(ret, &Condition{
					Type:               condition.Type,
					Status:             string(condition.Status),
					Reason:             condition.Reason,
					Message:            condition.Message,
					LastTransitionTime: t,
				})
			}
			return ret
		}(),
	}, nil
}

func (SQLDatabase) IsPersistence()        {}
func (i SQLDatabase) GetName() string     { return i.Name }
func (i SQLDatabase) GetID() scalar.Ident { return i.ID }

func IsHealthy(cs []v1alpha1.Condition) bool {
	for _, cond := range cs {
		if cond.Type == string(corev1.PodReady) && cond.Reason == "UpToDate" && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
