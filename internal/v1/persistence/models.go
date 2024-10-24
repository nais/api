package persistence

import (
	"github.com/nais/api/internal/v1/graphv1/modelv1"
)

type Persistence interface {
	modelv1.Node
	IsPersistence()
}
