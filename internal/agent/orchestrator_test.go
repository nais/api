package agent

import (
	"testing"

	"github.com/nais/api/internal/agent/chat"
	"github.com/sirupsen/logrus"
)

func TestParseChartToolCall(t *testing.T) {
	o := &Orchestrator{
		log: logrus.NewEntry(logrus.StandardLogger()),
	}

	tests := []struct {
		name        string
		args        map[string]any
		wantErr     bool
		errContains string
		validate    func(*testing.T, *ChartData)
	}{
		{
			name: "valid chart with all fields",
			args: map[string]any{
				"chart_type":     "line",
				"title":          "CPU Usage",
				"environment":    "dev",
				"query":          "sum(rate(container_cpu_usage_seconds_total[5m]))",
				"interval":       "1h",
				"y_format":       "cpu_cores",
				"label_template": "{pod}/{container}",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.ChartType != "line" {
					t.Errorf("expected chart_type 'line', got %q", c.ChartType)
				}
				if c.Title != "CPU Usage" {
					t.Errorf("expected title 'CPU Usage', got %q", c.Title)
				}
				if c.Environment != "dev" {
					t.Errorf("expected environment 'dev', got %q", c.Environment)
				}
				if c.Query != "sum(rate(container_cpu_usage_seconds_total[5m]))" {
					t.Errorf("unexpected query: %q", c.Query)
				}
				if c.Interval != "1h" {
					t.Errorf("expected interval '1h', got %q", c.Interval)
				}
				if c.YFormat != "cpu_cores" {
					t.Errorf("expected y_format 'cpu_cores', got %q", c.YFormat)
				}
				if c.LabelTemplate != "{pod}/{container}" {
					t.Errorf("expected label_template '{pod}/{container}', got %q", c.LabelTemplate)
				}
			},
		},
		{
			name: "valid chart with required fields only",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "Memory Usage",
				"environment": "prod",
				"query":       "container_memory_usage_bytes",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.ChartType != "line" {
					t.Errorf("expected chart_type 'line', got %q", c.ChartType)
				}
				if c.Interval != "" {
					t.Errorf("expected empty interval, got %q", c.Interval)
				}
				if c.YFormat != "" {
					t.Errorf("expected empty y_format, got %q", c.YFormat)
				}
				if c.LabelTemplate != "" {
					t.Errorf("expected empty label_template, got %q", c.LabelTemplate)
				}
			},
		},
		{
			name: "missing chart_type",
			args: map[string]any{
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
			},
			wantErr:     true,
			errContains: "chart_type is required",
		},
		{
			name: "empty chart_type",
			args: map[string]any{
				"chart_type":  "",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
			},
			wantErr:     true,
			errContains: "chart_type is required",
		},
		{
			name: "unsupported chart_type",
			args: map[string]any{
				"chart_type":  "bar",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
			},
			wantErr:     true,
			errContains: "unsupported chart_type",
		},
		{
			name: "missing title",
			args: map[string]any{
				"chart_type":  "line",
				"environment": "dev",
				"query":       "some_query",
			},
			wantErr:     true,
			errContains: "title is required",
		},
		{
			name: "missing environment",
			args: map[string]any{
				"chart_type": "line",
				"title":      "CPU Usage",
				"query":      "some_query",
			},
			wantErr:     true,
			errContains: "environment is required",
		},
		{
			name: "missing query",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
			},
			wantErr:     true,
			errContains: "query is required",
		},
		{
			name: "invalid interval",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"interval":    "2h",
			},
			wantErr:     true,
			errContains: "invalid interval",
		},
		{
			name: "valid interval 6h",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"interval":    "6h",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "6h" {
					t.Errorf("expected interval '6h', got %q", c.Interval)
				}
			},
		},
		{
			name: "valid interval 1d",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"interval":    "1d",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "1d" {
					t.Errorf("expected interval '1d', got %q", c.Interval)
				}
			},
		},
		{
			name: "valid interval 7d",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"interval":    "7d",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "7d" {
					t.Errorf("expected interval '7d', got %q", c.Interval)
				}
			},
		},
		{
			name: "valid interval 30d",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"interval":    "30d",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "30d" {
					t.Errorf("expected interval '30d', got %q", c.Interval)
				}
			},
		},
		{
			name: "invalid y_format",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"y_format":    "invalid",
			},
			wantErr:     true,
			errContains: "invalid y_format",
		},
		{
			name: "valid y_format number",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"y_format":    "number",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "number" {
					t.Errorf("expected y_format 'number', got %q", c.YFormat)
				}
			},
		},
		{
			name: "valid y_format percentage",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"y_format":    "percentage",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "percentage" {
					t.Errorf("expected y_format 'percentage', got %q", c.YFormat)
				}
			},
		},
		{
			name: "valid y_format bytes",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"y_format":    "bytes",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "bytes" {
					t.Errorf("expected y_format 'bytes', got %q", c.YFormat)
				}
			},
		},
		{
			name: "valid y_format duration",
			args: map[string]any{
				"chart_type":  "line",
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
				"y_format":    "duration",
			},
			wantErr: false,
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "duration" {
					t.Errorf("expected y_format 'duration', got %q", c.YFormat)
				}
			},
		},
		{
			name: "wrong type for chart_type",
			args: map[string]any{
				"chart_type":  123,
				"title":       "CPU Usage",
				"environment": "dev",
				"query":       "some_query",
			},
			wantErr:     true,
			errContains: "chart_type is required",
		},
		{
			name: "wrong type for title",
			args: map[string]any{
				"chart_type":  "line",
				"title":       123,
				"environment": "dev",
				"query":       "some_query",
			},
			wantErr:     true,
			errContains: "title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolCall := chat.ToolCall{
				ID:        "test-id",
				Name:      renderChartToolName,
				Arguments: tt.args,
			}

			chart, err := o.parseChartToolCall(toolCall)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if chart == nil {
				t.Error("expected chart data, got nil")
				return
			}

			if tt.validate != nil {
				tt.validate(t, chart)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
