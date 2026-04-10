-- +goose Up
-- Remove activity log entries for CREATE_CREDENTIALS action with serviceType KAFKA as these should
-- not be created in the first place.
DELETE FROM activity_log_entries
WHERE
	action = 'CREATE_CREDENTIALS'
	AND (
		CONVERT_FROM(data, 'UTF8')::JSONB ->> 'serviceType'
	) = 'KAFKA'
;

-- For the remaining CREATE_CREDENTIALS entries, update the action to CREDENTIALS_CREATED and move
-- the serviceType and instanceName from the data field to the resource_type and resource_name
-- fields respectively. Also, remove the keys from the data field as they are now redundant.
UPDATE activity_log_entries
SET
	action = 'CREDENTIALS_CREATED',
	resource_type = (
		CONVERT_FROM(data, 'UTF8')::JSONB ->> 'serviceType'
	),
	resource_name = (
		CONVERT_FROM(data, 'UTF8')::JSONB ->> 'instanceName'
	),
	data = CONVERT_TO(
		(
			CONVERT_FROM(data, 'UTF8')::JSONB - 'serviceType' - 'instanceName'
		)::TEXT,
		'UTF8'
	)
WHERE
	action = 'CREATE_CREDENTIALS'
;
