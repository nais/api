-- name: ConversationExists :one
SELECT
	EXISTS (
		SELECT
			1
		FROM
			agent_conversations
		WHERE
			id = @id
			AND user_id = @user_id
	)
;

-- name: TouchConversation :exec
UPDATE agent_conversations
SET
	updated_at = NOW()
WHERE
	id = @id
;

-- name: CountConversations :one
SELECT
	COUNT(*)
FROM
	agent_conversations
WHERE
	user_id = @user_id
;

-- name: DeleteOldestConversations :exec
DELETE FROM agent_conversations
WHERE
	id IN (
		SELECT
			agent_conversations.id
		FROM
			agent_conversations
		WHERE
			agent_conversations.user_id = @user_id
		ORDER BY
			agent_conversations.created_at ASC
		LIMIT
			@limit_count
	)
;

-- name: CreateConversation :one
INSERT INTO
	agent_conversations (user_id, title)
VALUES
	(@user_id, @title)
RETURNING
	id
;

-- name: InsertMessage :exec
INSERT INTO
	agent_messages (conversation_id, role, content, tool_calls)
VALUES
	(@conversation_id, @role, @content, @tool_calls)
;

-- name: ListConversations :many
SELECT
	id,
	title,
	updated_at
FROM
	agent_conversations
WHERE
	user_id = @user_id
ORDER BY
	updated_at DESC
;

-- name: GetConversation :one
SELECT
	id,
	title,
	created_at,
	updated_at
FROM
	agent_conversations
WHERE
	id = @id
	AND user_id = @user_id
;

-- name: GetConversationMessages :many
SELECT
	id,
	role,
	content,
	tool_calls,
	created_at
FROM
	agent_messages
WHERE
	conversation_id = @conversation_id
ORDER BY
	created_at ASC
;

-- name: DeleteConversation :execrows
DELETE FROM agent_conversations
WHERE
	id = @id
	AND user_id = @user_id
;

-- name: GetConversationHistory :many
SELECT
	role,
	content,
	tool_calls,
	tool_call_id
FROM
	agent_messages
WHERE
	conversation_id = @conversation_id
ORDER BY
	created_at ASC
;
