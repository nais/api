package pagination

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCursor_MarshalGQLContext(t *testing.T) {
	tests := map[string]struct {
		c        Cursor
		expected string
	}{
		"v1 0": {
			c:        Cursor{},
			expected: `"42E5H9"`,
		},
		"v1 13": {
			c:        Cursor{Offset: 13},
			expected: `"ELQhtKc"`,
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
		"v1 0": {
			cursor:   "42E5H9",
			expected: &Cursor{},
		},
		"v1 13": {
			cursor:   "ELQhtKc",
			expected: &Cursor{Offset: 13},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &Cursor{}
			if err := c.UnmarshalGQLContext(context.TODO(), tc.cursor); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.expected, c, cmpopts.IgnoreUnexported(Cursor{})); diff != "" {
				t.Errorf("diff: -want +got\n%s", diff)
			}
		})
	}
}
