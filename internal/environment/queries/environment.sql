-- name: DeleteAllEnvironments :exec
DELETE FROM environments
;

-- name: InsertEnvironment :exec
INSERT INTO
	environments (name, gcp)
VALUES
	(@name, @gcp)
;
