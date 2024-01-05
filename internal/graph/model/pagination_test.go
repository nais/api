package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
	"github.com/stretchr/testify/assert"
)

func Test_NewPagination(t *testing.T) {
	t.Run("no pagination", func(t *testing.T) {
		pagination := model.NewPagination(nil, nil)
		assert.Equal(t, 0, pagination.Offset)
		assert.Equal(t, 20, pagination.Limit)
	})

	t.Run("pagination with values", func(t *testing.T) {
		pagination := model.NewPagination(intP(42), intP(1337))
		assert.Equal(t, 42, pagination.Offset)
		assert.Equal(t, 1337, pagination.Limit)
	})
}

func intP(i int) *int {
	return &i
}
