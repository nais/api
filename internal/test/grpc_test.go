package test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/grpc"
)

type googleMonitoringFake struct {
	monitoringpb.MetricServiceServer
}

var _ monitoringpb.MetricServiceServer = (*googleMonitoringFake)(nil)

func TestExampleGoogleMonitoringFake(t *testing.T) {
	g := &googleMonitoringFake{}

	server := StartGRPCServer(func(s *grpc.Server) {
		monitoringpb.RegisterMetricServiceServer(s, g)
	})
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := monitoring.NewMetricClient(ctx, server.ClientOptions()...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	it := client.ListTimeSeries(ctx, &monitoringpb.ListTimeSeriesRequest{
		Name:   "project123/instance1",
		Filter: `metric.type="cloudsql.googleapis.com/database/cpu/utilization" AND resource.type="cloudsql_database"`,
	})
	timeSeries := make([]*monitoringpb.TimeSeries, 0)
	for {
		met, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		timeSeries = append(timeSeries, met)
	}
	if len(timeSeries) != 1 {
		t.Fatalf("expected 1 time series, got %d", len(timeSeries))
	}

	if expected := "cloudsql.googleapis.com/database/cpu/utilization"; timeSeries[0].Metric.Type != expected {
		t.Fatalf("expected metric type %q, got %q", expected, timeSeries[0].Metric.Type)
	}

	if expected := 0.5; timeSeries[0].Points[0].Value.GetDoubleValue() != expected {
		t.Fatalf("expected value %f, got %f", expected, timeSeries[0].Points[0].Value.GetDoubleValue())
	}
}

func (g *googleMonitoringFake) ListTimeSeries(_ context.Context, request *monitoringpb.ListTimeSeriesRequest) (*monitoringpb.ListTimeSeriesResponse, error) {
	timeSeries := make([]*monitoringpb.TimeSeries, 0)
	ts := &monitoringpb.TimeSeries{
		Metric:    &metric.Metric{},
		Points:    make([]*monitoringpb.Point, 0),
		ValueType: metric.MetricDescriptor_DOUBLE,
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
		ts.Points = append(ts.Points, &monitoringpb.Point{
			Value: &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: 0.5,
				},
			},
		})

		ts.Resource.Labels["database_id"] = "project123:instance1"
		timeSeries = append(timeSeries, ts)
	}
	return &monitoringpb.ListTimeSeriesResponse{
		TimeSeries: timeSeries,
	}, nil
}
