package main

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/sirupsen/logrus"
)

const (
	teamsConnString = "postgres://gammel:gammel@localhost:3009/gammel?sslmode=disable"
	newConnString   = "postgres://api:api@localhost:3002/api?sslmode=disable"
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

	runTeams(ctx, db)
	runConsoleBackend(ctx, db)
}
