package model

import (
	"strings"

	"github.com/nais/api/internal/database/gensql"
)

func (t ResourceType) ToDatabaseEnum() gensql.ResourceType {
	return gensql.ResourceType(strings.ToLower(string(t)))
}
