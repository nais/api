-- name: CreateUsersyncRun :exec
INSERT INTO usersync_runs (id, started_at, finished_at, error)
VALUES (@id, @started_at, @finished_at, @error);

-- name: GetUsersyncRunsCount :one
SELECT COUNT(*) FROM usersync_runs;

-- name: GetUsersyncRuns :many
SELECT * FROM usersync_runs
ORDER BY started_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
