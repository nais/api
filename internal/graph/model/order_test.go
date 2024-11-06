package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
)

func TestCompare(t *testing.T) {
	tests := map[string]struct {
		a, b      string
		direction model.OrderDirection
		expected  int
	}{
		"asc sorted int":      {a: "a", b: "b", direction: model.OrderDirectionAsc, expected: -1},
		"asc not sorted int":  {a: "b", b: "a", direction: model.OrderDirectionAsc, expected: 1},
		"desc sorted int":     {a: "a", b: "b", direction: model.OrderDirectionDesc, expected: 1},
		"desc not sorted int": {a: "b", b: "a", direction: model.OrderDirectionDesc, expected: -1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := model.Compare(tt.a, tt.b, tt.direction); got != tt.expected {
				t.Errorf("Expected %d, got: %v", tt.expected, got)
			}
		})
	}
}
