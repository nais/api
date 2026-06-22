package scalar

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type Date time.Time

func (d Date) MarshalGQLContext(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, strconv.Quote(d.String()))
	if err != nil {
		return fmt.Errorf("writing date: %w", err)
	}
	return nil
}

func (d *Date) UnmarshalGQLContext(_ context.Context, v any) error {
	date, ok := v.(string)
	if !ok {
		return fmt.Errorf("date must be a string")
	}

	if date == "" {
		return fmt.Errorf("date must not be empty")
	}

	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return fmt.Errorf("invalid date format: %q", date)
	}

	*d = Date(t)
	return nil
}

// MarshalJSON encodes the Date as a quoted YYYY-MM-DD string, matching its
// GraphQL representation. This is needed because `type Date time.Time` does not
// inherit time.Time's JSON methods.
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(d.String())), nil
}

// UnmarshalJSON decodes a quoted YYYY-MM-DD string into a Date.
func (d *Date) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("date must be a quoted string: %w", err)
	}

	if s == "" {
		return nil
	}

	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return fmt.Errorf("invalid date format: %q", s)
	}

	*d = Date(t)
	return nil
}

// NewDate returns a Date from a time.Time
func NewDate(t time.Time) Date {
	return Date(t.UTC())
}

// String returns the Date as a string
func (d Date) String() string {
	return time.Time(d).Format(time.DateOnly)
}

// Time returns the Date as a time.Time instance
func (d Date) Time() time.Time {
	return time.Time(d)
}

func (d *Date) PgDate() pgtype.Date {
	if d == nil {
		return pgtype.Date{}
	}

	return pgtype.Date{
		Time:  d.Time(),
		Valid: !d.Time().IsZero(),
	}
}
