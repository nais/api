-- +goose Up
UPDATE audit_events
SET
	action = 'ADDED'
WHERE
	action = 'TEAM_MEMBER_ADDED'
	AND resource_type = 'TEAM_MEMBER'
;

UPDATE audit_events
SET
	action = 'REMOVED'
WHERE
	action = 'TEAM_MEMBER_REMOVED'
	AND resource_type = 'TEAM_MEMBER'
;

-- +goose Down
UPDATE audit_events
SET
	action = 'TEAM_MEMBER_ADDED'
WHERE
	action = 'ADDED'
	AND resource_type = 'TEAM_MEMBER'
;

UPDATE audit_events
SET
	action = 'TEAM_MEMBER_REMOVED'
WHERE
	action = 'REMOVED'
	AND resource_type = 'TEAM_MEMBER'
;
