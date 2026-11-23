package pagination

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestKeysetCursorDoesNotChangeOffsetPagination(t *testing.T) {
	cursor := Cursor{Keyset: "opaque-page-token"}
	var buffer bytes.Buffer
	if err := cursor.MarshalGQLContext(t.Context(), &buffer); err != nil {
		t.Fatal(err)
	}
	var encoded string
	if err := json.Unmarshal(buffer.Bytes(), &encoded); err != nil {
		t.Fatal(err)
	}
	parsed := Cursor{Offset: 13}
	if err := parsed.UnmarshalGQLContext(t.Context(), encoded); err != nil || parsed.Keyset != cursor.Keyset || parsed.Offset != 0 {
		t.Fatalf("keyset cursor roundtrip: %+v, %v", parsed, err)
	}
	if _, err := ParsePage(new(1), &parsed, nil, nil); err == nil {
		t.Fatal("keyset cursor was accepted by offset pagination")
	}
	if err := parsed.UnmarshalGQLContext(t.Context(), "ELQhtKc"); err != nil || parsed.Offset != 13 || parsed.Keyset != "" {
		t.Fatalf("legacy cursor roundtrip changed: %+v, %v", parsed, err)
	}
}

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
