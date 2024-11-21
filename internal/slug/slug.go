package slug

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type Slug string

func (s *Slug) UnmarshalGQLContext(_ context.Context, v any) error {
	input, ok := v.(string)
	if !ok {
		return fmt.Errorf("slug must be a string")
	}

	*s = Slug(strings.TrimSpace(input))
	return s.Validate()
}

func (s Slug) MarshalGQLContext(_ context.Context, w io.Writer) error {
	txt := strconv.Quote(s.String())
	_, err := io.WriteString(w, txt)
	return err
}

func (s Slug) String() string {
	return string(s)
}

type ErrInvalidSlug struct {
	Message string
}

func (e *ErrInvalidSlug) Error() string {
	return e.Message
}

func (e *ErrInvalidSlug) GraphError() string {
	return e.Message
}

// reservedSlugs is a list of slugs that are reserved and cannot be used for NAIS teams.
var reservedSlugs = []Slug{
	"nais-system",
	"kube-system",
	"kube-node-lease",
	"kube-public",
	"kyverno",
	"cnrm-system",
	"configconnector-operator-system",
}

var slugPattern = regexp.MustCompile(`^[a-z](-?[a-z0-9]+)+$`)

func (s Slug) Validate() error {
	for _, reserved := range reservedSlugs {
		if s == reserved {
			return invalid("This slug is reserved by NAIS.")
		}
	}

	if strings.HasPrefix(s.String(), "team") {
		return invalid("The name prefix 'team' is redundant. When you create a team, it is by definition a team. Try again with a different name, perhaps just removing the prefix?")
	}

	if len(s) < 3 {
		return invalid("A team slug must be at least 3 characters long.")
	}

	if len(s) > 30 {
		return invalid("A team slug must be at most 30 characters long.")
	}

	if !slugPattern.MatchString(s.String()) {
		return invalid("A team slug must match the following pattern: %q.", slugPattern.String())
	}

	return nil
}

func invalid(format string, a ...any) error {
	return &ErrInvalidSlug{Message: fmt.Sprintf(format, a...)}
}
