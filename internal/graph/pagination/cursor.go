package pagination

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/btcsuite/btcutil/base58"
	"github.com/nais/api/internal/graph/apierror"
)

var cursorVersions = map[string]func(c *Cursor, i []byte) error{
	"v1": parseCursorV1,
	"v2": func(c *Cursor, token []byte) error {
		if len(token) == 0 || len(token) > 16384 {
			return apierror.Errorf("Invalid keyset cursor")
		}
		c.Keyset = string(token)
		return nil
	},
}

type Cursor struct {
	Offset int    `json:"offset"`
	Keyset string `json:"-"`
	empty  bool
}

func (c *Cursor) valid() bool {
	return c != nil && !c.empty
}

func (c Cursor) MarshalGQLContext(_ context.Context, w io.Writer) error {
	b := []byte{'v', '1', ':'}
	if c.Keyset != "" {
		b = append([]byte("v2:"), c.Keyset...)
	} else {
		b = strconv.AppendInt(b, int64(c.Offset), 10)
	}

	_, _ = w.Write([]byte{'"'})
	_, err := w.Write([]byte(base58.Encode(b)))
	_, _ = w.Write([]byte{'"'})
	return err
}

func (c *Cursor) UnmarshalGQLContext(_ context.Context, v any) error {
	*c = Cursor{}
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
