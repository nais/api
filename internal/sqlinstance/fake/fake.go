package fake

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/test"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

type FakeGoogleAPI struct {
	ClientGRPCOptions []option.ClientOption
	ClientHTTPOptions []option.ClientOption
	instanceLister    SQLInstanceLister
	server            *test.GRPCServer
	monitoringpb.MetricServiceServer
}

var _ monitoringpb.MetricServiceServer = (*FakeGoogleAPI)(nil)

type SQLInstanceLister func() ([]*model.SQLInstance, error)

type FakeOption func(*FakeGoogleAPI)

type ClusterInformers struct {
	informers k8s.ClusterInformers
}

func NewFakeGoogleAPI(opts ...FakeOption) (*FakeGoogleAPI, error) {
	m := &FakeGoogleAPI{}
	for _, opt := range opts {
		opt(m)
	}

	if m.instanceLister == nil {
		m.instanceLister = func() ([]*model.SQLInstance, error) {
			return []*model.SQLInstance{}, nil
		}
	}

	server := test.StartGRPCServer(func(s *grpc.Server) {
		monitoringpb.RegisterMetricServiceServer(s, m)
	})

	m.server = server
	m.ClientGRPCOptions = server.ClientOptions()
	m.ClientHTTPOptions = []option.ClientOption{
		option.WithHTTPClient(&http.Client{
			Transport: m.SqlAdminApi(),
		}),
	}
	return m, nil
}

func WithInformerInstanceLister(infs k8s.ClusterInformers) FakeOption {
	return func(m *FakeGoogleAPI) {
		m.instanceLister = (&ClusterInformers{informers: infs}).ListSqlInstances
	}
}

func WithInstanceLister(lister SQLInstanceLister) FakeOption {
	return func(m *FakeGoogleAPI) {
		m.instanceLister = lister
	}
}

func (i *ClusterInformers) ListSqlInstances() ([]*model.SQLInstance, error) {
	instances := make([]*model.SQLInstance, 0)
	for env, informer := range i.informers {
		objs, err := informer.SqlInstance.Lister().List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			instance, err := model.ToSqlInstance(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, err
			}
			instances = append(instances, instance)
		}
	}
	return instances, nil
}

func (f FakeGoogleAPI) Close() {
	f.server.Close()
}

func (f FakeGoogleAPI) ListTimeSeries(_ context.Context, request *monitoringpb.ListTimeSeriesRequest) (*monitoringpb.ListTimeSeriesResponse, error) {
	instances, err := f.instanceLister()
	if err != nil {
		return nil, err
	}

	timeSeries := make([]*monitoringpb.TimeSeries, 0)
	for _, instance := range instances {
		ts := &monitoringpb.TimeSeries{
			Metric:    &metric.Metric{},
			Points:    make([]*monitoringpb.Point, 0),
			ValueType: metric.MetricDescriptor_DOUBLE, // or  metric.MetricDescriptor_INT64
			Resource: &monitoredres.MonitoredResource{
				Labels: map[string]string{
					"database_id": "",
				},
			},
		}

		switch {
		case strings.Contains(request.Filter, "cpu/utilization"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/cpu/utilization"
			ts.ValueType = metric.MetricDescriptor_DOUBLE
			addDoublePoint(ts, 0.5)
		case strings.Contains(request.Filter, "cpu/reserved_cores"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/cpu/reserved_cores"
			ts.ValueType = metric.MetricDescriptor_INT64
			addInt64Point(ts, 1)
		case strings.Contains(request.Filter, "disk/utilization"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/disk/utilization"
			ts.ValueType = metric.MetricDescriptor_DOUBLE
			addDoublePoint(ts, 0.3)
		case strings.Contains(request.Filter, "disk/quota"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/disk/quota"
			ts.ValueType = metric.MetricDescriptor_INT64
			addInt64Point(ts, 1000000000)
		case strings.Contains(request.Filter, "memory/utilization"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/memory/utilization"
			ts.ValueType = metric.MetricDescriptor_DOUBLE
			addDoublePoint(ts, 0.8)
		case strings.Contains(request.Filter, "memory/quota"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/memory/quota"
			ts.ValueType = metric.MetricDescriptor_INT64
			addInt64Point(ts, 4000000000)
		}

		ts.Resource.Labels["database_id"] = instance.ProjectID + ":" + instance.Name
		timeSeries = append(timeSeries, ts)
	}
	return &monitoringpb.ListTimeSeriesResponse{
		TimeSeries: timeSeries,
	}, nil
}

func (f FakeGoogleAPI) SqlAdminApi() RoundTripFunc {
	return func(req *http.Request) *http.Response {
		parts := strings.Split(req.URL.Path, "/")
		last := parts[len(parts)-1]
		instances := make([]*sqladmin.DatabaseInstance, 0)
		var resp any

		// Example all instances path: /v1/projects/nais-dev-cdea/instances
		switch last {
		case "instances":
			inst, err := f.instanceLister()
			if err != nil {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       nil,
				}
			}

			for _, i := range inst {
				instances = append(instances, &sqladmin.DatabaseInstance{Name: i.Name, State: "RUNNABLE"})
			}
			resp = &sqladmin.InstancesListResponse{
				Items: instances,
			}
		case "users":
			resp = &sqladmin.UsersListResponse{
				Items: []*sqladmin.User{
					{
						Name: "foo",
					},
					{
						Name: "bar",
					},
				},
			}

		default:
			resp = &sqladmin.DatabaseInstance{
				Name:  last,
				State: "RUNNABLE",
			}
		}

		body, err := json.Marshal(resp)
		if err != nil {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(err.Error())),
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(string(body))),
		}
	}
}

func addInt64Point(ts *monitoringpb.TimeSeries, value int64) {
	ts.Points = append(ts.Points, &monitoringpb.Point{
		Value: &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: value,
			},
		},
	})
}

func addDoublePoint(ts *monitoringpb.TimeSeries, value float64) {
	ts.Points = append(ts.Points, &monitoringpb.Point{
		Value: &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{
				DoubleValue: value,
			},
		},
	})
}

type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
