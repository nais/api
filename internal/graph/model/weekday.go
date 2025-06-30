package model

import (
	"fmt"
	"io"
	"strconv"
)

// The days of the week.
type Weekday string

const (
	// Monday
	WeekdayMonday Weekday = "MONDAY"
	// Tuesday
	WeekdayTuesday Weekday = "TUESDAY"
	// Wednesday
	WeekdayWednesday Weekday = "WEDNESDAY"
	// Thursday
	WeekdayThursday Weekday = "THURSDAY"
	// Friday
	WeekdayFriday Weekday = "FRIDAY"
	// Saturday
	WeekdaySaturday Weekday = "SATURDAY"
	// Sunday
	WeekdaySunday Weekday = "SUNDAY"
)

var AllWeekday = []Weekday{
	WeekdayMonday,
	WeekdayTuesday,
	WeekdayWednesday,
	WeekdayThursday,
	WeekdayFriday,
	WeekdaySaturday,
	WeekdaySunday,
}

func (e Weekday) IsValid() bool {
	switch e {
	case WeekdayMonday, WeekdayTuesday, WeekdayWednesday, WeekdayThursday, WeekdayFriday, WeekdaySaturday, WeekdaySunday:
		return true
	}
	return false
}

func (e Weekday) String() string {
	return string(e)
}

func (e *Weekday) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = Weekday(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid Weekday", str)
	}
	return nil
}

func (e Weekday) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
