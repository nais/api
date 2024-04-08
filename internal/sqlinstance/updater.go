package sqlinstance

import (
	"context"
	"fmt"
	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"time"
)

type Updater struct {
	k8sClient *k8s.Client
	metrics   *Metrics
	db        database.Database
	log       logrus.FieldLogger
}

type SQLInstanceWithEnvAndNamespace struct {
	Env       string
	Namespace string
	ProjectId string
	Instance  *sql_cnrm_cloud_google_com_v1beta1.SQLInstance
}

func RunUpdater(ctx context.Context, k8sClient *k8s.Client, db database.Database, log logrus.FieldLogger) error {
	updater := NewUpdater(ctx, k8sClient, db, log)
	if updater == nil {
		log.Error("failed to create updater")
		return nil
	}
	_, err := updater.UpdateMetrics(ctx)
	if err != nil {
		log.WithError(err).Error("failed to update Metrics")
	}
	return nil
}

func NewUpdater(ctx context.Context, k8sClient *k8s.Client, db database.Database, log logrus.FieldLogger) *Updater {
	metrics, err := NewMetrics(ctx, log)
	if err != nil {
		log.WithError(err).Error("failed to create Metrics client")
		return nil
	}
	return &Updater{
		k8sClient: k8sClient,
		metrics:   metrics,
		db:        db,
		log:       log,
	}
}

func WithMetrics(metrics *Metrics) func(*Updater) {
	return func(u *Updater) {
		u.metrics = metrics
	}
}

func (u *Updater) UpdateMetrics(ctx context.Context) (rowsUpserted int, err error) {
	start := time.Now()
	instances, err := u.allSqlInstances()
	if err != nil {
		return 0, err
	}

	for _, instance := range instances {
		metrics, err := u.allMetricsFor(ctx, instance.ProjectId, instance.ProjectId+":"+instance.Instance.Name)
		if err != nil {
			return 0, err
		}

		u.log.WithField("duration", time.Since(start)).Debugf("got Metrics for %s", instance.Instance.Name)
		u.log.Debugf("Metrics: %v", metrics)
	}

	return 0, nil
}

func (u *Updater) allSqlInstances() ([]*SQLInstanceWithEnvAndNamespace, error) {
	ret := make([]*SQLInstanceWithEnvAndNamespace, 0)
	for env, infs := range u.k8sClient.Informers() {
		objs, err := infs.SqlInstanceInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("list SQL instances: %w", err)
		}

		for _, obj := range objs {
			sqlInstance := &sql_cnrm_cloud_google_com_v1beta1.SQLInstance{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, sqlInstance); err != nil {
				return nil, fmt.Errorf("converting to SQL instance: %w", err)
			}

			projectId := sqlInstance.GetAnnotations()["cnrm.cloud.google.com/project-id"]
			if projectId == "" {
				return nil, fmt.Errorf("missing project ID annotation")
			}

			ret = append(ret, &SQLInstanceWithEnvAndNamespace{
				Env:       env,
				Namespace: sqlInstance.GetNamespace(),
				ProjectId: projectId,
				Instance:  sqlInstance,
			})
		}
	}
	return ret, nil
}

func (u *Updater) allMetricsFor(ctx context.Context, projectId, dbId string) (map[MetricType]float64, error) {
	metricTypes := []MetricType{CpuUtilization, CpuCores, MemoryUtilization, MemoryQuota, DiskUtilization, DiskQuota}
	metrics := make(map[MetricType]float64)
	for _, t := range metricTypes {
		m, err := u.metrics.AverageFor(ctx, projectId, WithQuery(t, dbId))
		if err != nil {
			return nil, err
		}
		metrics[t] = m
	}
	return metrics, nil
}
