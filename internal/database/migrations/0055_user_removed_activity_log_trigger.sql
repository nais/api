-- +goose Up
-- Add trigger to create activity log when a user is deleted (e.g. by usersync)
-- When the user is deleted, we should get all user_roles for the user where a team_slug is not null
-- and create an activity log for each team the user was a member of

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_user_deletion()
RETURNS TRIGGER AS $$
BEGIN
		INSERT INTO activity_log_entries (actor, action, resource_type, resource_name, team_slug, data, environment)
		SELECT DISTINCT
				'system' AS actor,
				'REMOVED' AS action,
				'TEAM' AS resource_type,
				ur.target_team_slug AS resource_name,
				ur.target_team_slug AS team_slug,
				jsonb_build_object(
						'userID', OLD.id,
						'userEmail', OLD.email
				)::text::bytea AS data,
				NULL AS environment
		FROM user_roles ur
		WHERE ur.user_id = OLD.id AND ur.target_team_slug IS NOT NULL;

		RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS user_deletion_trigger ON users;
CREATE TRIGGER user_deletion_trigger
BEFORE DELETE ON users
FOR EACH ROW
EXECUTE FUNCTION log_user_deletion();
