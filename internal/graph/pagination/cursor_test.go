package pagination

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"

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

func TestCursor_TimeRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		ts     time.Time
		offset int
	}{
		{time.Unix(0, 1792570000123456789), 0},
		{time.Unix(0, 1792570000123456789), 4999},
		{time.Unix(0, 0), 1},
	} {
		buf := &bytes.Buffer{}
		if err := NewTimeCursor(tc.ts, tc.offset).MarshalGQLContext(context.TODO(), buf); err != nil {
			t.Fatal(err)
		}

		c := &Cursor{}
		if err := c.UnmarshalGQLContext(context.TODO(), strings.Trim(buf.String(), `"`)); err != nil {
			t.Fatal(err)
		}
		got, ok := c.Time()
		if !ok {
			t.Fatal("expected time cursor")
		}
		if !got.Equal(tc.ts) || got.UnixNano() != tc.ts.UnixNano() {
			t.Errorf("expected %d, got %d", tc.ts.UnixNano(), got.UnixNano())
		}
		if c.Offset != tc.offset {
			t.Errorf("expected offset %d, got %d", tc.offset, c.Offset)
		}
		if c.IsEmpty() {
			t.Error("expected non-empty cursor")
		}
	}
}

func TestCursor_OffsetCursorHasNoTime(t *testing.T) {
	c := &Cursor{}
	if err := c.UnmarshalGQLContext(context.TODO(), "ELQhtKc"); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Time(); ok {
		t.Error("expected offset cursor without time")
	}
	var nilCursor *Cursor
	if _, ok := nilCursor.Time(); ok || !nilCursor.IsEmpty() {
		t.Error("expected nil cursor to be empty and without time")
	}
	empty := &Cursor{}
	if err := empty.UnmarshalGQLContext(context.TODO(), ""); err != nil || !empty.IsEmpty() {
		t.Errorf("expected empty cursor, got %v", err)
	}
}

func TestCursor_InvalidTimeCursor(t *testing.T) {
	for _, raw := range []string{"t1:", "t1:123", "t1:abc:0", "t1:123:abc", "t1:123:-1", "t1:123:99999999999"} {
		c := &Cursor{}
		if err := c.UnmarshalGQLContext(context.TODO(), base58.Encode([]byte(raw))); err == nil {
			t.Errorf("expected error for %q", raw)
		}
	}
}
