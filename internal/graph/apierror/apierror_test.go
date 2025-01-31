package apierror_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
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
		if contains := "some error"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error message to contain %q, got %q", contains, err)
		}
	})

	t.Run("database error", func(t *testing.T) {
		defer hook.Reset()

		databaseError := &pgconn.PgError{Message: "some database error"}
		err := testWithError(databaseError)
		if contains := "The database encountered an error"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error message to contain %q, got %q", contains, err)
		}

		if len(hook.Entries) != 1 {
			t.Errorf("expected 1 log entry, got %d", len(hook.Entries))
		}

		if hook.LastEntry().Level != logrus.ErrorLevel {
			t.Errorf("expected log level to be %q, got %q", logrus.ErrorLevel, hook.LastEntry().Level)
		}

		if contains := "some database error"; !strings.Contains(hook.LastEntry().Message, contains) {
			t.Errorf("expected log message to contain %q, got %q", contains, hook.LastEntry().Message)
		}

		e := hook.LastEntry()
		fieldData, exists := e.Data[logrus.ErrorKey]
		if !exists {
			t.Fatalf("expected log entry to contain error field")
		}

		attachedErr, ok := fieldData.(error)
		if !ok {
			t.Fatalf("unable to cast to error")
		}

		if !errors.Is(attachedErr, databaseError) {
			t.Fatalf("invalid error type: expected %T, got %T", databaseError, attachedErr)
		}
	})

	t.Run("no rows from SQL query", func(t *testing.T) {
		defer hook.Reset()

		err := testWithError(sql.ErrNoRows)
		if contains := "Object was not found"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error message to contain %q, got %q", contains, err)
		}
	})

	t.Run("context canceled", func(t *testing.T) {
		defer hook.Reset()

		err := testWithError(context.Canceled)
		if contains := "Request canceled"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error message to contain %q, got %q", contains, err)
		}
	})

	t.Run("unhandled error", func(t *testing.T) {
		defer hook.Reset()

		unhandlerError := errors.New("some unhandled error")
		err := testWithError(unhandlerError)
		if contains := "we didn't write a suitable error message"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error message to contain %q, got %q", contains, err)
		}

		if len(hook.Entries) != 1 {
			t.Errorf("expected 1 log entry, got %d", len(hook.Entries))
		}

		if hook.LastEntry().Level != logrus.ErrorLevel {
			t.Errorf("expected log level to be %q, got %q", logrus.ErrorLevel, hook.LastEntry().Level)
		}

		if contains := "some unhandled error"; !strings.Contains(hook.LastEntry().Message, contains) {
			t.Errorf("expected log message to contain %q, got %q", contains, hook.LastEntry().Message)
		}

		fieldData, exists := hook.LastEntry().Data[logrus.ErrorKey]
		if !exists {
			t.Fatalf("expected log entry to contain error field")
		}

		attachedErr, ok := fieldData.(error)
		if !ok {
			t.Fatalf("unable to cast to error")
		}

		if !errors.Is(attachedErr, unhandlerError) {
			t.Fatalf("invalid error type: expected %T, got %T", unhandlerError, attachedErr)
		}
	})
}
