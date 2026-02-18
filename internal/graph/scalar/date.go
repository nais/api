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
