package modelv1_test

import (
	"testing"

	"github.com/nais/api/internal/v1/graphv1/modelv1"
)

func TestCompare(t *testing.T) {
	tests := map[string]struct {
		a, b      string
		direction modelv1.OrderDirection
		expected  int
	}{
		"asc sorted int":      {a: "a", b: "b", direction: modelv1.OrderDirectionAsc, expected: -1},
		"asc not sorted int":  {a: "b", b: "a", direction: modelv1.OrderDirectionAsc, expected: 1},
		"desc sorted int":     {a: "a", b: "b", direction: modelv1.OrderDirectionDesc, expected: 1},
		"desc not sorted int": {a: "b", b: "a", direction: modelv1.OrderDirectionDesc, expected: -1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := modelv1.Compare(tt.a, tt.b, tt.direction); got != tt.expected {
				t.Errorf("Expected %d, got: %v", tt.expected, got)
			}
		})
	}
}
