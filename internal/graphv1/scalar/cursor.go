package scalar

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
)

var cursorVersions = map[string]func(c *Cursor, i []byte) error{
	"v1": parseCursorV1,
}

type Cursor struct {
	Offset int64 `json:"offset"`
	Limit  int64 `json:"limit"`
}

func (c Cursor) MarshalGQLContext(_ context.Context, w io.Writer) error {
	b := []byte{'v', '1', ':'}
	b = strconv.AppendInt(b, c.Offset, 10)
	b = append(b, ':')
	b = strconv.AppendInt(b, c.Limit, 10)

	// Base64 encode
	b64 := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
	base64.StdEncoding.Encode(b64, b)

	_, err := w.Write(b64)
	return err
}

func (c *Cursor) UnmarshalGQLContext(_ context.Context, v interface{}) error {
	if s, ok := v.(string); ok {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return fmt.Errorf("invalid cursor format: %w", err)
		}
		version, cursor, ok := bytes.Cut(b, []byte{':'})
		if !ok {
			return fmt.Errorf("invalid cursor format")
		}
		parseCursor, ok := cursorVersions[string(version)]
		if !ok {
			return fmt.Errorf("unsupported cursor version")
		}
		if err := parseCursor(c, cursor); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("invalid cursor type")
}

func parseCursorV1(c *Cursor, b []byte) error {
	offsetb, limitb, ok := bytes.Cut(b, []byte{':'})
	if !ok {
		return fmt.Errorf("invalid cursor v1 format")
	}

	offset, err := strconv.ParseInt(string(offsetb), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid cursor v1 offset: %w", err)
	}

	limit, err := strconv.ParseInt(string(limitb), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid cursor v1 limit: %w", err)
	}

	c.Offset = offset
	c.Limit = limit

	return nil
}
