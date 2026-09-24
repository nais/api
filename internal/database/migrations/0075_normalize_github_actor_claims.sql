-- +goose Up
-- Activity log data predates the OIDC claim struct and uses camelCase keys.
-- Normalize those keys to match the names used when decoding GitHub tokens.
WITH entries AS (
	SELECT
		id,
		CONVERT_FROM(data, 'UTF8')::JSONB AS payload
	FROM
		activity_log_entries
	WHERE
		data IS NOT NULL
),
renamed AS (
	SELECT
		id,
		JSONB_SET(
			payload,
			'{gitHubActorClaims}',
			(
				SELECT
					JSONB_OBJECT_AGG(
						CASE claim.key
							WHEN 'actorId' THEN 'actor_id'
							WHEN 'baseRef' THEN 'base_ref'
							WHEN 'checkRunId' THEN 'check_run_id'
							WHEN 'eventName' THEN 'event_name'
							WHEN 'headRef' THEN 'head_ref'
							WHEN 'jobWorkflowRef' THEN 'job_workflow_ref'
							WHEN 'jobWorkflowSha' THEN 'job_workflow_sha'
							WHEN 'refType' THEN 'ref_type'
							WHEN 'repositoryId' THEN 'repository_id'
							WHEN 'repositoryOwner' THEN 'repository_owner'
							WHEN 'repositoryOwnerId' THEN 'repository_owner_id'
							WHEN 'repositoryVisibility' THEN 'repository_visibility'
							WHEN 'runAttempt' THEN 'run_attempt'
							WHEN 'runId' THEN 'run_id'
							WHEN 'runnerEnvironment' THEN 'runner_environment'
							WHEN 'runNumber' THEN 'run_number'
							WHEN 'workflowRef' THEN 'workflow_ref'
							WHEN 'workflowSha' THEN 'workflow_sha'
							ELSE claim.key
						END,
						claim.value
						ORDER BY STRPOS(claim.key, '_') > 0
					)
				FROM
					JSONB_EACH(payload -> 'gitHubActorClaims') AS claim(key, value)
			)
		) AS payload
	FROM
		entries
	WHERE
		JSONB_TYPEOF(payload -> 'gitHubActorClaims') = 'object'
		AND (payload -> 'gitHubActorClaims') ?| ARRAY[
			'actorId',
			'baseRef',
			'checkRunId',
			'eventName',
			'headRef',
			'jobWorkflowRef',
			'jobWorkflowSha',
			'refType',
			'repositoryId',
			'repositoryOwner',
			'repositoryOwnerId',
			'repositoryVisibility',
			'runAttempt',
			'runId',
			'runnerEnvironment',
			'runNumber',
			'workflowRef',
			'workflowSha'
		]
)
UPDATE activity_log_entries AS entry
SET
	data = CONVERT_TO(renamed.payload::TEXT, 'UTF8')
FROM
	renamed
WHERE
	entry.id = renamed.id
;
