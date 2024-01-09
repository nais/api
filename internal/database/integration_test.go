//go:build db_integration_test
// +build db_integration_test

package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"

	db "github.com/nais/api/internal/database"
	"github.com/stretchr/testify/assert"
)

const (
	databaseConnectRetries = 3
	databaseName           = "teams_backend_test"
	connString             = "postgresql://postgres:postgres@localhost:5666?sslmode=disable"
	connStringWithDb       = connString + "&dbname=" + databaseName
)

func TestUsers(t *testing.T) {
	ctx := context.Background()
	database, err := setupTestDatabase(ctx)
	if err != nil {
		t.Fatalf("Unable to setup database for integration tests: %v", err)
	}

	t.Run("Create and get user", func(t *testing.T) {
		const (
			name       = "User Name"
			email      = "user@examle.com"
			externalID = "external-id-123"
		)

		user, err := database.CreateUser(ctx, name, email, externalID)
		assert.NoError(t, err)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, externalID, user.ExternalID)

		user, err = database.GetUserByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, externalID, user.ExternalID)

		user, err = database.GetUserByEmail(ctx, email)
		assert.NoError(t, err)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, externalID, user.ExternalID)

		user, err = database.GetUserByExternalID(ctx, externalID)
		assert.NoError(t, err)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, externalID, user.ExternalID)
	})
}

func setupTestDatabase(ctx context.Context) (db.Database, error) {
	if err := createEmptyTestDatabase(ctx); err != nil {
		return nil, err
	}
	return db.New(ctx, connStringWithDb)
}

func createEmptyTestDatabase(ctx context.Context) error {
	conn, err := connect(ctx, connString, databaseConnectRetries)
	if err != nil {
		return fmt.Errorf("unable to connect to postgres, it is probably not running. Start it with ´make start-integration-test-db´: %w", err)
	}
	defer conn.Close(ctx)

	if _, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", databaseName)); err != nil {
		return fmt.Errorf("dropping existing database: %w", err)
	}

	if _, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", databaseName)); err != nil {
		return fmt.Errorf("creating database: %w", err)
	}

	return nil
}

func connect(ctx context.Context, connString string, connectRetries int) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	var conn *pgx.Conn
	for i := 0; i < connectRetries; i++ {
		conn, err = pgx.ConnectConfig(ctx, config)
		if err == nil {
			break
		}

		time.Sleep(time.Second * time.Duration(i+1))
	}

	if conn == nil {
		return nil, fmt.Errorf("giving up connecting to the database after %d attempts: %w", connectRetries, err)
	}

	return conn, nil
}
