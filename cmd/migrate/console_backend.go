package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runConsoleBackend(ctx context.Context, db *pgxpool.Pool, teams *pgx.Conn) {
	console, err := pgx.Connect(ctx, consoleConnString)
	if err != nil {
		log.Fatalf("failed to connect to console database: %s", err)
	}

	teamSlugs := []string{}

	rows, err := teams.Query(ctx, "SELECT slug FROM teams")
	if err != nil {
		log.Fatalf("failed to fetch teams: %s", err)
	}

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			log.Fatalf("failed to scan team: %s", err)
		}
		teamSlugs = append(teamSlugs, slug)
	}

	slices.Sort(teamSlugs)

	if err := moveResourceMetrics(ctx, db, console, teamSlugs); err != nil {
		log.Fatalf("failed to move resource metrics: %s", err)
	}

	if err := moveCost(ctx, db, console, teamSlugs); err != nil {
		log.Fatalf("failed to move cost: %s", err)
	}
}

func moveResourceMetrics(ctx context.Context, db *pgxpool.Pool, console *pgx.Conn, teamSlugs []string) error {
	start := time.Now()
	fmt.Println("Moving resouce metrics")
	defer func() {
		fmt.Println("  Done moving resouce metrics in", time.Since(start))
	}()

	rows, err := console.Query(ctx, `
	SELECT
		id, timestamp, env, team, app, resource_type, usage, request
	FROM resource_utilization_metrics`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	notFoundTeams := []string{}

	batch := &pgx.Batch{}
	for rows.Next() {
		var (
			id            int64
			timestamp     time.Time
			env           string
			team          *string
			app           string
			resource_type string
			usage         float64
			request       float64
		)

		if err := rows.Scan(&id, &timestamp, &env, &team, &app, &resource_type, &usage, &request); err != nil {
			return err
		}

		if team != nil && !slices.Contains(teamSlugs, *team) {
			if !slices.Contains(notFoundTeams, *team) {
				notFoundTeams = append(notFoundTeams, *team)
			}
			continue
		}

		args := []any{id, timestamp, env, team, app, resource_type, usage, request}
		batch.Queue(`
		INSERT INTO resource_utilization_metrics (id, timestamp, environment, team_slug, app, resource_type, usage, request)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
		`, args...)

		if batch.Len() >= 1500 {
			err := conn.SendBatch(ctx, batch).Close()
			if err != nil {
				fmt.Println("Error sending resource metrics batch", err)
			}
			batch = &pgx.Batch{}
		}
	}

	if batch.Len() > 0 {
		err := conn.SendBatch(ctx, batch).Close()
		if err != nil {
			fmt.Println("Error finalizing resource metrics batch", err)
		}
	}

	if len(notFoundTeams) > 0 {
		fmt.Println("  Teams not found:", notFoundTeams)
	}

	return nil
}

func moveCost(ctx context.Context, db *pgxpool.Pool, console *pgx.Conn, teamSlugs []string) error {
	start := time.Now()
	fmt.Println("Moving cost")
	defer func() {
		fmt.Println("  Done moving cost in", time.Since(start))
	}()

	rows, err := console.Query(ctx, `
	SELECT
		id, env, team, app, cost_type, date, daily_cost
	FROM cost`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	notFoundTeams := []string{}

	batch := &pgx.Batch{}
	for rows.Next() {
		var (
			id        int64
			env       *string
			team      *string
			app       string
			costType  string
			date      pgtype.Date
			dailyCost float64
		)

		if err := rows.Scan(&id, &env, &team, &app, &costType, &date, &dailyCost); err != nil {
			return err
		}

		if team != nil && !slices.Contains(teamSlugs, *team) {
			if !slices.Contains(notFoundTeams, *team) {
				notFoundTeams = append(notFoundTeams, *team)
			}
			continue
		}

		args := []any{id, env, team, app, costType, date, dailyCost}
		batch.Queue(`
		INSERT INTO cost (id, environment, team_slug, app, cost_type, date, daily_cost)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
		`, args...)

		if batch.Len() >= 1500 {
			err := conn.SendBatch(ctx, batch).Close()
			if err != nil {
				fmt.Println("Error sending cost batch", err)
			}
			batch = &pgx.Batch{}
		}
	}

	if batch.Len() > 0 {
		err := conn.SendBatch(ctx, batch).Close()
		if err != nil {
			fmt.Println("Error finalizing cost batch", err)
		}
	}

	if len(notFoundTeams) > 0 {
		fmt.Println("  Teams not found:", notFoundTeams)
	}

	return nil
}
