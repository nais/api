package elevation

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type Elevation struct {
	ID              ident.Ident
	Type            ElevationType
	TeamSlug        slug.Slug
	EnvironmentName string
	ResourceName    string
	UserEmail       string
	Reason          string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

func (Elevation) IsNode() {}

type ElevationType string

const (
	ElevationTypeSecret      ElevationType = "SECRET"
	ElevationTypeExec        ElevationType = "INSTANCE_EXEC"
	ElevationTypePortForward ElevationType = "INSTANCE_PORT_FORWARD"
	ElevationTypeDebug       ElevationType = "INSTANCE_DEBUG"
)

func (e ElevationType) IsValid() bool {
	switch e {
	case ElevationTypeSecret, ElevationTypeExec, ElevationTypePortForward, ElevationTypeDebug:
		return true
	}
	return false
}

func (e ElevationType) String() string {
	return string(e)
}

func (e *ElevationType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ElevationType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ElevationType", str)
	}
	return nil
}

func (e ElevationType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ElevationInput struct {
	Type            ElevationType
	Team            slug.Slug
	EnvironmentName string
	ResourceName    string
}

type CreateElevationInput struct {
	Type            ElevationType
	Team            slug.Slug
	EnvironmentName string
	ResourceName    string
	Reason          string
	DurationMinutes int
}

type CreateElevationPayload struct {
	Elevation *Elevation
}
