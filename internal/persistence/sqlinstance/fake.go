package sqlinstance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/test"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/grpc"
)

type FakeGoogleAPI struct {
	instances         *watcher.Watcher[*SQLInstance]
	ClientGRPCOptions []option.ClientOption
	ClientHTTPOptions []option.ClientOption
	server            *test.GRPCServer
	monitoringpb.MetricServiceServer
}

var _ monitoringpb.MetricServiceServer = (*FakeGoogleAPI)(nil)

func newFakeGoogleAPI(instances *watcher.Watcher[*SQLInstance]) (*FakeGoogleAPI, error) {
	m := &FakeGoogleAPI{
		instances: instances,
	}

	server := test.StartGRPCServer(func(s *grpc.Server) {
		monitoringpb.RegisterMetricServiceServer(s, m)
	})

	m.server = server
	m.ClientGRPCOptions = server.ClientOptions()
	m.ClientHTTPOptions = []option.ClientOption{
		option.WithHTTPClient(&http.Client{
			Transport: m.sqlAdminAPI(),
		}),
	}
	return m, nil
}

func (f FakeGoogleAPI) Close() {
	f.server.Close()
}

func (f FakeGoogleAPI) ListTimeSeries(_ context.Context, request *monitoringpb.ListTimeSeriesRequest) (*monitoringpb.ListTimeSeriesResponse, error) {
	if request.Aggregation != nil {
		return f.aggregatedTimeSeries(request)
	}

	instances := f.instances.All()

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

		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		switch {
		case strings.Contains(request.Filter, "cpu/utilization"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/cpu/utilization"
			ts.ValueType = metric.MetricDescriptor_DOUBLE
			addDoublePoint(ts, r.Float64())
		case strings.Contains(request.Filter, "cpu/reserved_cores"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/cpu/reserved_cores"
			ts.ValueType = metric.MetricDescriptor_INT64
			addInt64Point(ts, r.Int63())
		case strings.Contains(request.Filter, "disk/utilization"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/disk/utilization"
			ts.ValueType = metric.MetricDescriptor_DOUBLE
			addDoublePoint(ts, r.Float64())
		case strings.Contains(request.Filter, "disk/quota"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/disk/quota"
			ts.ValueType = metric.MetricDescriptor_INT64
			addInt64Point(ts, 1000000000)
		case strings.Contains(request.Filter, "memory/utilization"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/memory/utilization"
			ts.ValueType = metric.MetricDescriptor_DOUBLE
			addDoublePoint(ts, r.Float64())
		case strings.Contains(request.Filter, "memory/quota"):
			ts.Metric.Type = "cloudsql.googleapis.com/database/memory/quota"
			ts.ValueType = metric.MetricDescriptor_INT64
			addInt64Point(ts, 4000000000)
		}

		ts.Resource.Labels["database_id"] = instance.Obj.ProjectID + ":" + instance.Obj.Name
		timeSeries = append(timeSeries, ts)
	}
	return &monitoringpb.ListTimeSeriesResponse{
		TimeSeries: timeSeries,
	}, nil
}

func (f FakeGoogleAPI) aggregatedTimeSeries(request *monitoringpb.ListTimeSeriesRequest) (*monitoringpb.ListTimeSeriesResponse, error) {
	min := 0.0
	max := 10.0
	switch {
	case strings.Contains(request.Filter, "cpu/reserved_cores"):
		min = 5.0
		max = 10.0
	case strings.Contains(request.Filter, "cpu/usage_time"):
		min = 0.0
		max = 5.0
	case strings.Contains(request.Filter, "memory/quota"):
		min = 4000000000
		max = 40000000000
	case strings.Contains(request.Filter, "memory/total_usage"):
		min = 2000000000
		max = 4000000000
	case strings.Contains(request.Filter, "disk/quota"):
		min = 1000000000
		max = 10000000000
	case strings.Contains(request.Filter, "disk/bytes_used"):
		min = 500000000
		max = 1000000000
	}

	return &monitoringpb.ListTimeSeriesResponse{
		TimeSeries: []*monitoringpb.TimeSeries{
			{
				Metric: &metric.Metric{},
				Points: []*monitoringpb.Point{
					{
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: min + rand.Float64()*(max-min),
							},
						},
					},
				},
			},
		},
	}, nil
}

func (f FakeGoogleAPI) sqlAdminAPI() RoundTripFunc {
	return func(req *http.Request) *http.Response {
		parts := strings.Split(req.URL.Path, "/")
		last := parts[len(parts)-1]
		var resp any
		projectID := parts[3]

		// Example all instances path: /v1/projects/nais-dev-cdea/instances
		switch last {
		case "instances":
			instances := make([]*sqladmin.DatabaseInstance, 0)
			inst := f.instances.All()

			for _, i := range inst {
				instances = append(instances, &sqladmin.DatabaseInstance{Name: i.Obj.Name, State: "RUNNABLE", Project: projectID})
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
				Name:    last,
				Project: projectID,
				State:   "RUNNABLE",
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
