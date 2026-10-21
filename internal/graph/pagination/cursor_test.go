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

func TestCursor_PositionedTimeRoundTrip(t *testing.T) {
	want := NewPositionedTimeCursor(time.Unix(0, 1792570000123456789), 2, 17)
	buf := &bytes.Buffer{}
	if err := want.MarshalGQLContext(context.TODO(), buf); err != nil {
		t.Fatal(err)
	}

	got := &Cursor{}
	if err := got.UnmarshalGQLContext(context.TODO(), strings.Trim(buf.String(), `"`)); err != nil {
		t.Fatal(err)
	}
	gotTime, hasTime := got.Time()
	gotPosition, hasPosition := got.Position()
	if !hasTime || !gotTime.Equal(time.Unix(0, 1792570000123456789)) || got.Offset != 2 {
		t.Fatalf("unexpected time cursor: %+v, %v", got, gotTime)
	}
	if !hasPosition || gotPosition != 17 {
		t.Fatalf("expected absolute position 17, got %d (present: %v)", gotPosition, hasPosition)
	}
}

func TestCursor_StableTimeRoundTrip(t *testing.T) {
	want := NewStableTimeCursor(time.Unix(0, 1792570000123456789), 2, "labels\x00value\x00message", 3).WithPosition(17)
	buf := &bytes.Buffer{}
	if err := want.MarshalGQLContext(context.TODO(), buf); err != nil {
		t.Fatal(err)
	}

	got := &Cursor{}
	if err := got.UnmarshalGQLContext(context.TODO(), strings.Trim(buf.String(), `"`)); err != nil {
		t.Fatal(err)
	}
	gotTime, hasTime := got.Time()
	position, hasPosition := got.Position()
	anchor, occurrence, hasAnchor := got.Anchor()
	if !hasTime || !gotTime.Equal(time.Unix(0, 1792570000123456789)) || got.Offset != 2 {
		t.Fatalf("unexpected stable time cursor: %+v, %v", got, gotTime)
	}
	if !hasPosition || position != 17 {
		t.Fatalf("expected position 17, got %d (present: %v)", position, hasPosition)
	}
	wantAnchor, _, _ := want.Anchor()
	if !hasAnchor || anchor != wantAnchor || occurrence != 3 {
		t.Fatalf("expected anchor %q occurrence 3, got %q occurrence %d", wantAnchor, anchor, occurrence)
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
	for _, raw := range []string{"t1:", "t1:123", "t1:abc:0", "t1:123:abc", "t1:123:-1", "t1:123:99999999999", "t2:123:0", "t2:123:-1:1", "t2:123:0:0", "t2:123:0:abc", "t2:123:0:2147483648", "t3:1:2:3:0:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "t3:1:2:3:1:bad"} {
		c := &Cursor{}
		if err := c.UnmarshalGQLContext(context.TODO(), base58.Encode([]byte(raw))); err == nil {
			t.Errorf("expected error for %q", raw)
		}
	}
}

func TestPageInfo_TimeCursorPositions(t *testing.T) {
	start := NewPositionedTimeCursor(time.Unix(0, 1), 0, 3)
	end := NewPositionedTimeCursor(time.Unix(0, 2), 0, 5)
	pageInfo := PageInfo{StartCursor: &start, EndCursor: &end}
	if pageInfo.PageStart() != 3 || pageInfo.PageEnd() != 5 {
		t.Errorf("expected page positions 3-5, got %d-%d", pageInfo.PageStart(), pageInfo.PageEnd())
	}
}
