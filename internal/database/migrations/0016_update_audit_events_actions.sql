-- +goose Up
UPDATE audit_events
SET
	action = 'CREATED'
WHERE
	action = 'TEAM_CREATED'
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'SYNCHRONIZED'
WHERE
	action = 'TEAM_SYNCHRONIZED'
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'ADDED'
WHERE
	action = 'TEAM_MEMBER_ADDED'
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'REMOVED'
WHERE
	action = 'TEAM_MEMBER_REMOVED'
	AND resource_type = 'TEAM'
;

-- +goose Down
UPDATE audit_events
SET
	action = 'TEAM_CREATED'
WHERE
	action = 'CREATED'
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'TEAM_SYNCHRONIZED'
WHERE
	action = 'SYNCHRONIZED'
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'TEAM_MEMBER_ADDED'
WHERE
	action = 'ADDED'
	AND resource_type = 'TEAM'
;

UPDATE audit_events
SET
	action = 'TEAM_MEMBER_REMOVED'
WHERE
	action = 'REMOVED'
	AND resource_type = 'TEAM'
;
