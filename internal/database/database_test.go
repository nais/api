package database_test

import (
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/nais/api/sqlc/schemas"
	"github.com/stretchr/testify/assert"
)

func TestCheckForDuplicateMigrations(t *testing.T) {
	_, err := iofs.New(schemas.FS, ".")
	assert.NoError(t, err)
}
