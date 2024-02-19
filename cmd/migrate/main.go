package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/sirupsen/logrus"
)

const (
	teamsConnString   = "" // postgres://console@[PROJECT_ID].iam@localhost:6000/console?sslmode=disable
	consoleConnString = "" // postgres://console-backend@[PROJECT_ID].iam@localhost:7000/console_backend?sslmode=disable
	newConnString     = "" // postgres://nais_api:[PASSWORD]@localhost:5432/nais_api?sslmode=disable
)

var environments = []string{"dev"}

func main() {
	ctx := context.Background()

	log := logrus.New()
	_, close, db, err := database.NewWithRaw(ctx, newConnString, log)
	if err != nil {
		log.Fatalf("failed to connect to database: %s", err)
	}
	defer close()

	teams, err := pgx.Connect(ctx, teamsConnString)
	if err != nil {
		log.Fatalf("failed to connect to teams database: %s", err)
	}

	runTeams(ctx, db, teams)
	runConsoleBackend(ctx, db, teams)
}
