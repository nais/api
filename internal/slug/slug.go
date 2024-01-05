package slug

import (
	"fmt"
	"io"
	"strconv"
)

type Slug string

// UnmarshalGQL implements the graphql.Unmarshaler interface
func (s *Slug) UnmarshalGQL(v interface{}) error {
	input, ok := v.(string)
	if !ok {
		return fmt.Errorf("slug must be a string")
	}

	*s = Slug(input)
	return nil
}

// MarshalGQL implements the graphql.Marshaler interface
func (s Slug) MarshalGQL(w io.Writer) {
	txt := strconv.Quote(s.String())
	io.WriteString(w, txt)
}

func (s Slug) String() string {
	return string(s)
}
