-- +goose Up
-- Remove "Team member" rows where the user already has "Team owner" for the same team.
-- This cleans up any existing duplicate role assignments before tightening the constraint.
DELETE FROM user_roles
WHERE
	id IN (
		SELECT
			ur_member.id
		FROM
			user_roles ur_member
			JOIN user_roles ur_owner ON ur_member.user_id = ur_owner.user_id
			AND ur_member.target_team_slug = ur_owner.target_team_slug
		WHERE
			ur_member.role_name = 'Team member'
			AND ur_owner.role_name = 'Team owner'
			AND ur_member.target_team_slug IS NOT NULL
	)
;

-- Drop the old index that includes role_name, allowing multiple roles per user per team.
DROP INDEX user_roles_user_id_role_name_target_team_slug_idx
;

-- Create a new stricter index enforcing exactly one role per user per team.
CREATE UNIQUE INDEX ON user_roles USING btree (user_id, target_team_slug)
WHERE
	(target_team_slug IS NOT NULL)
;
