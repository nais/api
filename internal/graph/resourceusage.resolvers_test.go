package graph_test

import (
	"context"
	"testing"
	"time"

	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/resourceusage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_queryResolver_ResourceUtilizationForApp(t *testing.T) {
	ctx := context.Background()

	t.Run("no dates specified", func(t *testing.T) {
		cpuData := make([]*model.ResourceUtilization, 0)
		memoryData := make([]*model.ResourceUtilization, 0)

		resourceUsageClient := resourceusage.NewMockClient(t)
		resourceUsageClient.
			EXPECT().
			ResourceUtilizationForApp(ctx, "env", "team", "app", mock.AnythingOfType("scalar.Date"), mock.AnythingOfType("scalar.Date")).
			Run(func(_ context.Context, _, _, _ string, start, end scalar.Date) {
				now := time.Now()
				today := now.Format(scalar.DateFormatYYYYMMDD)
				oneWeekAgo := now.AddDate(0, 0, -7).Format(scalar.DateFormatYYYYMMDD)
				assert.Equal(t, today, end.String())
				assert.Equal(t, oneWeekAgo, start.String())
			}).
			Return(&model.ResourceUtilizationForApp{
				CPU:    cpuData,
				Memory: memoryData,
			}, nil)

		resp, err := graph.
			NewResolver(nil, nil, nil, nil, resourceUsageClient, nil, nil, nil, nil).
			Query().
			ResourceUtilizationForApp(ctx, "env", "team", "app", nil, nil)
		assert.Equal(t, cpuData, resp.CPU)
		assert.Equal(t, memoryData, resp.Memory)
		assert.NoError(t, err)
	})

	t.Run("both dates specified", func(t *testing.T) {
		fromTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		from := scalar.NewDate(fromTime)
		toTime := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)
		to := scalar.NewDate(toTime)

		cpuData := make([]*model.ResourceUtilization, 0)
		memoryData := make([]*model.ResourceUtilization, 0)

		resourceUsageClient := resourceusage.NewMockClient(t)
		resourceUsageClient.
			EXPECT().
			ResourceUtilizationForApp(ctx, "env", "team", "app", mock.AnythingOfType("scalar.Date"), mock.AnythingOfType("scalar.Date")).
			Run(func(_ context.Context, _, _, _ string, start, end scalar.Date) {
				assert.Equal(t, fromTime.Format(scalar.DateFormatYYYYMMDD), start.String())
				assert.Equal(t, toTime.Format(scalar.DateFormatYYYYMMDD), end.String())
			}).
			Return(&model.ResourceUtilizationForApp{
				CPU:    cpuData,
				Memory: memoryData,
			}, nil)

		resp, err := graph.
			NewResolver(nil, nil, nil, nil, resourceUsageClient, nil, nil, nil, nil).
			Query().
			ResourceUtilizationForApp(ctx, "env", "team", "app", &from, &to)
		assert.NoError(t, err)
		assert.Equal(t, cpuData, resp.CPU)
		assert.Equal(t, memoryData, resp.Memory)
	})
}
