package persistence

import (
	"github.com/nais/api/internal/graph/model"
)

type Persistence interface {
	model.Node
	IsPersistence()
}
