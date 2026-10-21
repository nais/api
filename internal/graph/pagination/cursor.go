package pagination

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/nais/api/internal/graph/apierror"
)

var cursorVersions = map[string]func(c *Cursor, i []byte) error{
	"v1": parseCursorV1,
	"t1": parseCursorT1,
	"t2": parseCursorT2,
}

type Cursor struct {
	Offset int `json:"offset"`
	empty  bool

	// timeNanos and hasTime are set for time based cursors, see NewTimeCursor.
	timeNanos int64
	hasTime   bool
	position  int
	hasPos    bool
}

// NewTimeCursor creates a cursor pointing to a position in a time ordered result. The cursor refers to all items with
// a timestamp equal to or after t, excluding the first offset items with timestamp exactly t.
func NewTimeCursor(t time.Time, offset int) Cursor {
	return Cursor{
		Offset:    offset,
		timeNanos: t.UnixNano(),
		hasTime:   true,
	}
}

// NewPositionedTimeCursor creates a time cursor with its absolute 1-based position in the connection.
func NewPositionedTimeCursor(t time.Time, offset, position int) Cursor {
	c := NewTimeCursor(t, offset)
	c.position = position
	c.hasPos = true
	return c
}

// Time returns the timestamp of a time based cursor, and whether the cursor is time based.
func (c *Cursor) Time() (time.Time, bool) {
	if c == nil || !c.hasTime {
		return time.Time{}, false
	}
	return time.Unix(0, c.timeNanos).UTC(), true
}

// Position returns the absolute 1-based connection position of a positioned cursor.
func (c *Cursor) Position() (int, bool) {
	if c == nil || !c.hasPos {
		return 0, false
	}
	return c.position, true
}

// IsEmpty returns true if the cursor is nil or was provided as an empty string.
func (c *Cursor) IsEmpty() bool {
	return c == nil || c.empty
}

func (c *Cursor) valid() bool {
	return c != nil && !c.empty
}

func (c Cursor) MarshalGQLContext(_ context.Context, w io.Writer) error {
	var b []byte
	if c.hasTime {
		version := "t1:"
		if c.hasPos {
			version = "t2:"
		}
		b = []byte(version)
		b = strconv.AppendInt(b, c.timeNanos, 10)
		b = append(b, ':')
	} else {
		b = []byte{'v', '1', ':'}
	}
	b = strconv.AppendInt(b, int64(c.Offset), 10)
	if c.hasPos {
		b = append(b, ':')
		b = strconv.AppendInt(b, int64(c.position), 10)
	}

	_, _ = w.Write([]byte{'"'})
	_, err := w.Write([]byte(base58.Encode(b)))
	_, _ = w.Write([]byte{'"'})
	return err
}

func (c *Cursor) UnmarshalGQLContext(_ context.Context, v any) error {
	if s, ok := v.(string); ok {
		if s == "" {
			c.empty = true
			return nil
		}
		b := base58.Decode(s)
		version, cursor, ok := bytes.Cut(b, []byte{':'})
		if !ok {
			return apierror.Errorf("Unsupported cursor format")
		}
		parseCursor, ok := cursorVersions[string(version)]
		if !ok {
			return apierror.Errorf("Unsupported cursor version: %q", string(version))
		}
		if err := parseCursor(c, cursor); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("invalid cursor type")
}

func parseCursorV1(c *Cursor, offsetb []byte) error {
	offset, err := strconv.ParseInt(string(offsetb), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid cursor v1 offset: %w", err)
	}

	if offset > math.MaxInt32 {
		return fmt.Errorf("cursor v1 offset out of bounds")
	}

	c.Offset = int(offset)

	return nil
}

func parseCursorT1(c *Cursor, b []byte) error {
	ts, offset, ok := bytes.Cut(b, []byte{':'})
	if !ok {
		return apierror.Errorf("Unsupported cursor format")
	}

	nanos, err := strconv.ParseInt(string(ts), 10, 64)
	if err != nil {
		return apierror.Errorf("Unsupported cursor format")
	}

	if err := parseCursorV1(c, offset); err != nil {
		return apierror.Errorf("Unsupported cursor format")
	}

	if c.Offset < 0 {
		return apierror.Errorf("Unsupported cursor format")
	}

	c.timeNanos = nanos
	c.hasTime = true

	return nil
}

func parseCursorT2(c *Cursor, b []byte) error {
	ts, rest, ok := bytes.Cut(b, []byte{':'})
	if !ok {
		return apierror.Errorf("Unsupported cursor format")
	}
	offset, position, ok := bytes.Cut(rest, []byte{':'})
	if !ok {
		return apierror.Errorf("Unsupported cursor format")
	}

	nanos, err := strconv.ParseInt(string(ts), 10, 64)
	if err != nil {
		return apierror.Errorf("Unsupported cursor format")
	}
	if err := parseCursorV1(c, offset); err != nil || c.Offset < 0 {
		return apierror.Errorf("Unsupported cursor format")
	}
	pos, err := strconv.ParseInt(string(position), 10, 64)
	if err != nil || pos < 1 || pos > math.MaxInt32 {
		return apierror.Errorf("Unsupported cursor format")
	}

	c.timeNanos = nanos
	c.hasTime = true
	c.position = int(pos)
	c.hasPos = true
	return nil
}
