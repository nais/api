package apierror_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	ctx := context.Background()
	log, hook := test.NewNullLogger()
	presenterFunc := apierror.GetErrorPresenter(log)

	testWithError := func(err error) error {
		return presenterFunc(ctx, graphql.DefaultErrorPresenter(ctx, err))
	}

	t.Run("pre-formatted error message", func(t *testing.T) {
		defer hook.Reset()

		err := testWithError(apierror.Errorf("some error"))
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("database error", func(t *testing.T) {
		defer hook.Reset()

		databaseError := &pgconn.PgError{Message: "some database error"}
		err := testWithError(databaseError)
		assert.ErrorContains(t, err, apierror.ErrDatabase.Error())

		assert.Equal(t, 1, len(hook.Entries))
		assert.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
		assert.Contains(t, hook.LastEntry().Message, "some database error")

		e := hook.LastEntry()
		fieldData, exists := e.Data[logrus.ErrorKey]
		assert.True(t, exists)

		attachedErr, ok := fieldData.(error)
		assert.True(t, ok)

		assert.ErrorIs(t, attachedErr, databaseError)
	})

	t.Run("no rows from SQL query", func(t *testing.T) {
		defer hook.Reset()

		err := testWithError(sql.ErrNoRows)
		assert.ErrorContains(t, err, "Object was not found")
	})

	t.Run("context canceled", func(t *testing.T) {
		defer hook.Reset()

		err := testWithError(context.Canceled)
		assert.ErrorContains(t, err, "Request canceled")
	})

	t.Run("unhandled error", func(t *testing.T) {
		defer hook.Reset()

		unhandlerError := errors.New("some unhandled error")
		err := testWithError(unhandlerError)
		assert.ErrorContains(t, err, apierror.ErrInternal.Error())
		assert.Equal(t, 1, len(hook.Entries))
		assert.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
		assert.Contains(t, hook.LastEntry().Message, "some unhandled error")

		fieldData, exists := hook.LastEntry().Data[logrus.ErrorKey]
		assert.True(t, exists)

		attachedErr, ok := fieldData.(error)
		assert.True(t, ok)

		assert.ErrorIs(t, attachedErr, unhandlerError)
	})
}
