package scalar

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCursor_MarshalGQLContext(t *testing.T) {
	tests := map[string]struct {
		c        Cursor
		expected string
	}{
		"v1 0:0": {
			c:        Cursor{},
			expected: "djE6MDow",
		},
		"v1 13:14": {
			c:        Cursor{Offset: 13, Limit: 14},
			expected: "djE6MTM6MTQ=",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			if err := tc.c.MarshalGQLContext(context.TODO(), buf); err != nil {
				t.Fatal(err)
			}
			if buf.String() != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, buf.String())
			}
		})
	}
}

func TestCursor_UnmarshalGQLContext(t *testing.T) {
	tests := map[string]struct {
		cursor   string
		expected *Cursor
	}{
		"v1 0:0": {
			cursor:   "djE6MDow",
			expected: &Cursor{},
		},
		"v1 13:14": {
			cursor:   "djE6MTM6MTQ=",
			expected: &Cursor{Offset: 13, Limit: 14},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &Cursor{}
			if err := c.UnmarshalGQLContext(context.TODO(), tc.cursor); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.expected, c); diff != "" {
				t.Errorf("diff: -want +got\n%s", diff)
			}
		})
	}
}
