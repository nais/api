package tools

import (
	"context"
	"testing"
)

func TestRenderChart(t *testing.T) {
	ct := NewChartTools()
	ctx := context.Background()

	tests := []struct {
		name        string
		input       ChartData
		wantErr     bool
		errContains string
		validate    func(*testing.T, *ChartData)
	}{
		{
			name: "valid chart with all fields",
			input: ChartData{
				ChartType:     "line",
				Title:         "CPU Usage",
				Environment:   "dev",
				Query:         "sum(rate(container_cpu_usage_seconds_total[5m]))",
				Interval:      "1h",
				YFormat:       "cpu_cores",
				LabelTemplate: "{pod}/{container}",
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
			input: ChartData{
				ChartType:   "line",
				Title:       "Memory Usage",
				Environment: "prod",
				Query:       "container_memory_usage_bytes",
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
			name:        "missing chart_type",
			input:       ChartData{Title: "CPU Usage", Environment: "dev", Query: "some_query"},
			wantErr:     true,
			errContains: "chart_type is required",
		},
		{
			name:        "empty chart_type",
			input:       ChartData{ChartType: "", Title: "CPU Usage", Environment: "dev", Query: "some_query"},
			wantErr:     true,
			errContains: "chart_type is required",
		},
		{
			name:        "unsupported chart_type",
			input:       ChartData{ChartType: "bar", Title: "CPU Usage", Environment: "dev", Query: "some_query"},
			wantErr:     true,
			errContains: "unsupported chart_type",
		},
		{
			name:        "missing title",
			input:       ChartData{ChartType: "line", Environment: "dev", Query: "some_query"},
			wantErr:     true,
			errContains: "title is required",
		},
		{
			name:        "missing environment",
			input:       ChartData{ChartType: "line", Title: "CPU Usage", Query: "some_query"},
			wantErr:     true,
			errContains: "environment is required",
		},
		{
			name:        "missing query",
			input:       ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev"},
			wantErr:     true,
			errContains: "query is required",
		},
		{
			name:        "invalid interval",
			input:       ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", Interval: "2h"},
			wantErr:     true,
			errContains: "invalid interval",
		},
		{
			name:  "valid interval 6h",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", Interval: "6h"},
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "6h" {
					t.Errorf("expected interval '6h', got %q", c.Interval)
				}
			},
		},
		{
			name:  "valid interval 1d",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", Interval: "1d"},
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "1d" {
					t.Errorf("expected interval '1d', got %q", c.Interval)
				}
			},
		},
		{
			name:  "valid interval 7d",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", Interval: "7d"},
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "7d" {
					t.Errorf("expected interval '7d', got %q", c.Interval)
				}
			},
		},
		{
			name:  "valid interval 30d",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", Interval: "30d"},
			validate: func(t *testing.T, c *ChartData) {
				if c.Interval != "30d" {
					t.Errorf("expected interval '30d', got %q", c.Interval)
				}
			},
		},
		{
			name:        "invalid y_format",
			input:       ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", YFormat: "invalid"},
			wantErr:     true,
			errContains: "invalid y_format",
		},
		{
			name:  "valid y_format number",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", YFormat: "number"},
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "number" {
					t.Errorf("expected y_format 'number', got %q", c.YFormat)
				}
			},
		},
		{
			name:  "valid y_format percentage",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", YFormat: "percentage"},
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "percentage" {
					t.Errorf("expected y_format 'percentage', got %q", c.YFormat)
				}
			},
		},
		{
			name:  "valid y_format bytes",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", YFormat: "bytes"},
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "bytes" {
					t.Errorf("expected y_format 'bytes', got %q", c.YFormat)
				}
			},
		},
		{
			name:  "valid y_format cpu_cores",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", YFormat: "cpu_cores"},
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "cpu_cores" {
					t.Errorf("expected y_format 'cpu_cores', got %q", c.YFormat)
				}
			},
		},
		{
			name:  "valid y_format duration",
			input: ChartData{ChartType: "line", Title: "CPU Usage", Environment: "dev", Query: "q", YFormat: "duration"},
			validate: func(t *testing.T, c *ChartData) {
				if c.YFormat != "duration" {
					t.Errorf("expected y_format 'duration', got %q", c.YFormat)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chart, err := ct.RenderChart(ctx, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
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

func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
