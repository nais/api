-- +goose Up
-- Delete audit events that is no longer supported
-- We no longer update the secret itself, only the keys / values
DELETE FROM audit_events
WHERE
	action IN ('UPDATED')
	AND resource_type = 'SECRET'
;

-- We no longer support manually synchronizing a team
DELETE FROM audit_events
WHERE
	action IN ('SYNCHRONIZED')
	AND resource_type = 'TEAM'
;

-- We have these events, but previous ones are missing some required metadata
DELETE FROM audit_events
WHERE
	action = 'RESTARTED'
	AND resource_type = 'APP'
;

-- Update actions / resource types
UPDATE audit_events
SET
	action = 'DELETED'
WHERE
	action = 'DELETE_SECRET'
;

UPDATE audit_events
SET
	action = 'CREATED'
WHERE
	action = 'CREATE_SECRET'
;

UPDATE audit_events
SET
	action = 'UPDATED'
WHERE
	action IN (
		'TEAM_SET_PURPOSE',
		'TEAM_SET_DEFAULT_SLACK_CHANNEL',
		'TEAM_SET_ALERTS_SLACK_CHANNEL'
	)
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'SET_MEMBER_ROLE'
WHERE
	action = 'TEAM_MEMBER_SET_ROLE'
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'CREATE_DELETE_KEY'
WHERE
	action = 'TEAM_DELETION_REQUESTED'
;

UPDATE audit_events
SET
	action = 'CONFIRM_DELETE_KEY'
WHERE
	action = 'TEAM_DELETION_CONFIRMED'
;

UPDATE audit_events
SET
	resource_type = 'REPOSITORY'
WHERE
	resource_type = 'TEAM_REPOSITORY'
;

UPDATE audit_events
SET
	resource_type = 'TEAM'
WHERE
	resource_type = 'TEAM_MEMBER'
;

UPDATE audit_events
SET
	resource_type = 'JOB'
WHERE
	resource_type = 'NAISJOB'
;

UPDATE audit_events
SET
	action = 'UPDATED',
	resource_type = 'DEPLOY_KEY'
WHERE
	action = 'TEAM_DEPLOY_KEY_ROTATED'
;
