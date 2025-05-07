package maintenance

import (
	"time"

	"github.com/nais/api/internal/graph/ident"
)

type Update struct {
	Deadline          *time.Time `json:"deadline"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	DocumentationLink string     `json:"documentation_link"`
	StartAfter        *time.Time `json:"start_after"`
	StartAt           *time.Time `json:"start_at"`
}

type Maintenance struct {
	Identifier ident.Ident `json:"id"`
	Updates    []Update    `json:"updates"`
}

func (Maintenance) IsNode() {}
func (i *Maintenance) ID() ident.Ident {
	return i.Identifier
}
