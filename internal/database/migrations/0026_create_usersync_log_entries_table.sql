-- +goose Up
DROP TABLE usersync_runs
;

CREATE TYPE usersync_log_entry_action AS ENUM(
	'create_user',
	'update_user',
	'delete_user',
	'assign_role',
	'revoke_role'
)
;

CREATE TABLE usersync_log_entries (
	id UUID DEFAULT gen_random_uuid () PRIMARY KEY,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
	action usersync_log_entry_action NOT NULL,
	user_id UUID NOT NULL,
	user_name TEXT NOT NULL,
	user_email TEXT NOT NULL,
	old_user_name TEXT,
	old_user_email TEXT,
	role_name TEXT
)
;
