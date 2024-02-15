package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/slug"
)

func runTeams(ctx context.Context, db *pgxpool.Pool) {
	teams, err := pgx.Connect(ctx, teamsConnString)
	if err != nil {
		log.Fatalf("failed to connect to old database: %s", err)
	}

	if err := moveUsers(ctx, db, teams); err != nil {
		log.Fatalf("failed to move users: %s", err)
	}

	if err := moveTeams(ctx, db, teams); err != nil {
		log.Fatalf("failed to move teams: %s", err)
	}

	if err := moveServiceAccounts(ctx, db, teams); err != nil {
		log.Fatalf("failed to move service accounts: %s", err)
	}

	if err := moveUserRoles(ctx, db, teams); err != nil {
		log.Fatalf("failed to move user roles: %s", err)
	}

	if err := moveServiceAccountRoles(ctx, db, teams); err != nil {
		log.Fatalf("failed to move service account roles: %s", err)
	}

	if err := moveRepositoryAuthorizations(ctx, db, teams); err != nil {
		log.Fatalf("failed to move repository authorizations: %s", err)
	}

	if err := moveReconcilers(ctx, db, teams); err != nil {
		log.Fatalf("failed to move reconcilers: %s", err)
	}

	// TODO
	// if err := moveReconcilerStates(ctx, db, old); err != nil {
	// 	log.Fatalf("failed to move reconciler states: %s", err)
	// }

	if err := moveReconcilerOptOuts(ctx, db, teams); err != nil {
		log.Fatalf("failed to move reconciler opt outs: %s", err)
	}

	if err := moveReconcilerConfig(ctx, db, teams); err != nil {
		log.Fatalf("failed to move reconciler config: %s", err)
	}

	if err := moveAuditLogs(ctx, db, teams); err != nil {
		log.Fatalf("failed to move audit logs: %s", err)
	}
}

func moveUsers(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving users")
	defer func() {
		fmt.Println("  Done moving users in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		id, email, name, external_id
	FROM users`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			id         uuid.UUID
			email      string
			name       string
			externalID string
		)

		if err := rows.Scan(&id, &email, &name, &externalID); err != nil {
			return err
		}

		_, err := conn.Exec(ctx, `
		INSERT INTO users (id, email, name, external_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
		`, id, email, name, externalID)
		if err != nil {
			return err
		}

	}

	return nil
}

func moveTeams(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving teams")
	defer func() {
		fmt.Println("  Done moving teams in", time.Since(start))
	}()

	// TODO: Add Azure group email
	rows, err := old.Query(ctx, `
	SELECT
		teams.slug::text AS slug,
		teams.purpose AS purpose,
		teams.last_successful_sync,
		teams.slack_channel,
		gar.state->>'repopsitoryName' AS gar_repository,
		gh.state->>'slug' AS github_team_slug,
		gge.state->>'groupEmail' AS google_group_email
	FROM teams
	LEFT JOIN reconciler_states gar ON teams.slug = gar.team_slug AND gar.reconciler = 'google:gcp:gar'
	LEFT JOIN reconciler_states gh ON teams.slug = gh.team_slug AND gh.reconciler = 'github:team'
	LEFT JOIN reconciler_states gge ON teams.slug = gge.team_slug AND gge.reconciler = 'google:workspace-admin'
	`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			slug               slug.Slug
			purpose            string
			lastSuccessfulSync pgtype.Timestamp
			slackChannel       string
			garRepository      *string
			githubTeamSlug     *string
			googleGroupEmail   *string
		)

		if err := rows.Scan(&slug, &purpose, &lastSuccessfulSync, &slackChannel, &garRepository, &githubTeamSlug, &googleGroupEmail); err != nil {
			return err
		}

		_, err := conn.Exec(ctx, `
		INSERT INTO teams (slug, purpose, last_successful_sync, slack_channel, gar_repository, github_team_slug, google_group_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (slug) DO NOTHING
		`, slug, purpose, lastSuccessfulSync, slackChannel, garRepository, githubTeamSlug, googleGroupEmail)
		if err != nil {
			return err
		}
	}

	return nil
}

func moveUserRoles(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving user roles")
	defer func() {
		fmt.Println("  Done moving user roles in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		id, role_name, user_id, target_team_slug, target_service_account_id
	FROM user_roles`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			id                     int64
			roleName               string
			userID                 uuid.UUID
			targetTeamSlug         *slug.Slug
			targetServiceAccountID *uuid.UUID
		)

		if err := rows.Scan(&id, &roleName, &userID, &targetTeamSlug, &targetServiceAccountID); err != nil {
			return err
		}

		args := []any{id, roleName, userID, targetTeamSlug, targetServiceAccountID}
		_, err := conn.Exec(ctx, `
		INSERT INTO user_roles (id, role_name, user_id, target_team_slug, target_service_account_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
		`, args...)
		if err != nil {
			return fmt.Errorf("inserting user role, with fields %+v: %w", args, err)
		}
	}

	return nil
}

func moveServiceAccounts(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving service accounts")
	defer func() {
		fmt.Println("  Done moving service accounts in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		id, name
	FROM service_accounts`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			id   uuid.UUID
			name string
		)

		if err := rows.Scan(&id, &name); err != nil {
			return err
		}

		_, err := conn.Exec(ctx, `
		INSERT INTO service_accounts (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
		`, id, name)
		if err != nil {
			return err
		}
	}

	return nil
}

func moveServiceAccountRoles(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving service account roles")
	defer func() {
		fmt.Println("  Done moving service account roles in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		id, role_name, service_account_id, target_team_slug, target_service_account_id
	FROM service_account_roles`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			id                     int64
			roleName               string
			serviceAccountID       uuid.UUID
			targetTeamSlug         *slug.Slug
			targetServiceAccountID *uuid.UUID
		)

		if err := rows.Scan(&id, &roleName, &serviceAccountID, &targetTeamSlug, &targetServiceAccountID); err != nil {
			return err
		}

		args := []any{id, roleName, serviceAccountID, targetTeamSlug, targetServiceAccountID}
		_, err := conn.Exec(ctx, `
		INSERT INTO service_account_roles (id, role_name, service_account_id, target_team_slug, target_service_account_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
		`, args...)
		if err != nil {
			return fmt.Errorf("inserting service account role, with fields %+v: %w", args, err)
		}
	}

	return nil
}

func moveRepositoryAuthorizations(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving repository authorizations")
	defer func() {
		fmt.Println("  Done moving repository authorizations in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		team_slug, github_repository, repository_authorization
	FROM repository_authorizations`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			teamSlug   slug.Slug
			repository string
			repoAuth   string
		)

		if err := rows.Scan(&teamSlug, &repository, &repoAuth); err != nil {
			return err
		}

		args := []any{teamSlug, repository, repoAuth}
		_, err := conn.Exec(ctx, `
		INSERT INTO repository_authorizations (team_slug, github_repository, repository_authorization)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
		`, args...)
		if err != nil {
			return fmt.Errorf("inserting repository authorization, with fields %+v: %w", args, err)
		}
	}

	return nil
}

func moveReconcilers(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving reconcilers")
	defer func() {
		fmt.Println("  Done moving reconcilers in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		name, display_name, description, enabled
	FROM reconcilers`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			name        string
			displayName string
			description string
			enabled     bool
		)

		if err := rows.Scan(&name, &displayName, &description, &enabled); err != nil {
			return err
		}

		args := []any{name, displayName, description, enabled}
		_, err := conn.Exec(ctx, `
		INSERT INTO reconcilers (name, display_name, description, enabled)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
		`, args...)
		if err != nil {
			return fmt.Errorf("inserting reconciler, with fields %+v: %w", args, err)
		}
	}

	return nil
}

func moveReconcilerOptOuts(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving reconciler opt outs")
	defer func() {
		fmt.Println("  Done moving reconciler opt outs in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		team_slug, user_id, reconciler_name
	FROM reconciler_opt_outs`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			teamSlug   slug.Slug
			userID     uuid.UUID
			reconciler string
		)

		if err := rows.Scan(&teamSlug, &userID, &reconciler); err != nil {
			return err
		}

		args := []any{teamSlug, userID, reconciler}
		_, err := conn.Exec(ctx, `
		INSERT INTO reconciler_opt_outs (team_slug, user_id, reconciler_name)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
		`, args...)
		if err != nil {
			return fmt.Errorf("inserting reconciler opt out, with fields %+v: %w", args, err)
		}
	}

	return nil
}

func moveReconcilerConfig(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving reconciler config")
	defer func() {
		fmt.Println("  Done moving reconciler config in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		reconciler, key, display_name, description, value, secret
	FROM reconciler_config`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			reconciler  string
			key         string
			display     string
			description string
			value       *string
			secret      bool
		)

		if err := rows.Scan(&reconciler, &key, &display, &description, &value, &secret); err != nil {
			return err
		}

		args := []any{reconciler, key, display, description, value, secret}
		_, err := conn.Exec(ctx, `
		INSERT INTO reconciler_config (reconciler, key, display_name, description, value, secret)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING
		`, args...)
		if err != nil {
			return fmt.Errorf("inserting reconciler config, with fields %+v: %w", args, err)
		}
	}

	return nil
}

func moveAuditLogs(ctx context.Context, db *pgxpool.Pool, old *pgx.Conn) error {
	start := time.Now()
	fmt.Println("Moving audit logs")
	defer func() {
		fmt.Println("  Done moving audit logs in", time.Since(start))
	}()

	rows, err := old.Query(ctx, `
	SELECT
		id, created_at, correlation_id, component_name, actor, action, message, target_type, target_identifier
	FROM audit_logs`)
	if err != nil {
		return err
	}

	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for rows.Next() {
		var (
			id               uuid.UUID
			createdAt        time.Time
			correlationID    uuid.UUID
			componentName    string
			actor            *string
			action           string
			message          string
			targetType       string
			targetIdentifier string
		)

		if err := rows.Scan(&id, &createdAt, &correlationID, &componentName, &actor, &action, &message, &targetType, &targetIdentifier); err != nil {
			return err
		}

		args := []any{id, createdAt, correlationID, componentName, actor, action, message, targetType, targetIdentifier}
		_, err := conn.Exec(ctx, `
		INSERT INTO audit_logs (id, created_at, correlation_id, component_name, actor, action, message, target_type, target_identifier)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT DO NOTHING
		`, args...)
		if err != nil {
			return fmt.Errorf("inserting audit log, with fields %+v: %w", args, err)
		}

	}

	return nil
}
