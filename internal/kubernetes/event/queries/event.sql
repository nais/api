-- name: Upsert :batchexec
INSERT INTO
	k8s_events (
		uid,
		environment_name,
		involved_kind,
		involved_name,
		involved_namespace,
		data,
		reason,
		triggered_at
	)
VALUES
	(
		@uid,
		@environment_name,
		@involved_kind,
		@involved_name,
		@involved_namespace,
		@data,
		@reason,
		@triggered_at
	)
ON CONFLICT (uid) DO NOTHING
;
