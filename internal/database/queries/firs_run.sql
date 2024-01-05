-- name: IsFirstRun :one
SELECT first_run FROM first_run LIMIT 1;

-- name: FirstRunComplete :exec
UPDATE first_run SET first_run = false;
