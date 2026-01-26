# Agent Chat API - Client Documentation

This document describes the REST API for the Nais Agent chat service.

## Base Path

All agent endpoints are available under `/agent`.

## Authentication

The agent API uses the same authentication as the GraphQL API:
- Session cookies (OAuth2)
- JWT tokens via `Authorization: Bearer <token>` header
- API keys via `Authorization: Bearer <api-key>` header

All endpoints require authentication.

## Endpoints

### POST /agent/chat

Non-streaming chat endpoint. Sends a message and waits for the complete response.

**Request Body:**
```json
{
  "message": "How do I deploy my application?",
  "conversation_id": "optional-uuid-to-continue-conversation",
  "context": {
    "path": "/team/my-team/app/my-app",
    "team": "my-team",
    "app": "my-app",
    "env": "dev"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `message` | string | Yes | The user's message |
| `conversation_id` | string | No | UUID of existing conversation to continue |
| `context` | object | No | Current UI context to help the assistant |
| `context.path` | string | No | Current page path |
| `context.team` | string | No | Current team slug |
| `context.app` | string | No | Current application name |
| `context.env` | string | No | Current environment |

**Response:**
```json
{
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "message_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "content": "To deploy your application, you need to...",
  "tools_used": [
    {
      "name": "query_nais_api",
      "description": "Executed query_nais_api"
    }
  ],
  "sources": [
    {
      "title": "Deploying Applications",
      "url": "https://docs.nais.io/deploy/"
    }
  ]
}
```

---

### POST /agent/chat/stream

Streaming chat endpoint using Server-Sent Events (SSE). Sends a message and streams the response in real-time.

**Request Body:** Same as `/agent/chat`

**Response:** `Content-Type: text/event-stream`

The response is a stream of SSE events. Each event has a `data` field containing a JSON object.

#### Event Types

**metadata** - Sent first, contains conversation info:
```json
{
  "type": "metadata",
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "message_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
}
```

**content** - Streamed content chunks:
```json
{
  "type": "content",
  "content": "To deploy"
}
```

**tool_start** - Tool execution started:
```json
{
  "type": "tool_start",
  "tool_name": "query_nais_api",
  "description": "Executing query_nais_api..."
}
```

**tool_end** - Tool execution completed:
```json
{
  "type": "tool_end",
  "tool_name": "query_nais_api",
  "description": "Executed query_nais_api",
  "tool_success": true
}
```

**sources** - Documentation sources used:
```json
{
  "type": "sources",
  "sources": [
    {
      "title": "Deploying Applications",
      "url": "https://docs.nais.io/deploy/"
    }
  ]
}
```

**done** - Stream complete:
```json
{
  "type": "done"
}
```

**error** - An error occurred:
```json
{
  "type": "error",
  "error_code": "stream_error",
  "error_message": "Description of the error"
}
```

#### Example SSE Stream

```
data: {"type":"metadata","conversation_id":"550e8400-e29b-41d4-a716-446655440000","message_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8"}

data: {"type":"tool_start","tool_name":"query_nais_api","description":"Executing query_nais_api..."}

data: {"type":"tool_end","tool_name":"query_nais_api","description":"Executed query_nais_api","tool_success":true}

data: {"type":"content","content":"Based on "}

data: {"type":"content","content":"the information"}

data: {"type":"content","content":" I found..."}

data: {"type":"sources","sources":[{"title":"Deploying Applications","url":"https://docs.nais.io/deploy/"}]}

data: {"type":"done"}
```

---

### GET /agent/conversations

List all conversations for the authenticated user.

**Response:**
```json
{
  "conversations": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "How do I deploy my application?",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

---

### GET /agent/conversations/{conversationID}

Get a specific conversation with all messages.

**Response:**
```json
{
  "conversation": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "title": "How do I deploy my application?",
    "messages": [
      {
        "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
        "role": "user",
        "content": "How do I deploy my application?",
        "created_at": "2024-01-15T10:30:00Z"
      },
      {
        "id": "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
        "role": "assistant",
        "content": "To deploy your application...",
        "tools_used": [
          {
            "name": "query_nais_api",
            "description": "Executed query_nais_api"
          }
        ],
        "created_at": "2024-01-15T10:30:05Z"
      }
    ],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:05Z"
  }
}
```

---

### DELETE /agent/conversations/{conversationID}

Delete a conversation.

**Response:**
```json
{
  "deleted": true
}
```

---

## Error Responses

All endpoints return errors in the following format:

```json
{
  "error": "Description of the error"
}
```

**HTTP Status Codes:**
- `400 Bad Request` - Invalid request body or parameters
- `401 Unauthorized` - Authentication required
- `404 Not Found` - Conversation not found
- `500 Internal Server Error` - Server error

---

## Client Implementation Notes

### Handling SSE Streams

When consuming the streaming endpoint:

1. Set appropriate headers:
   ```
   Accept: text/event-stream
   Cache-Control: no-cache
   ```

2. Parse each `data:` line as JSON
3. Accumulate `content` events to build the full response
4. Handle `tool_start`/`tool_end` events for UI feedback
5. Stop processing when you receive `done` or `error`

### Conversation Management

- Conversations are automatically created when you don't provide a `conversation_id`
- Each user can have up to 10 conversations; older ones are automatically deleted
- Conversation titles are generated from the first message

### Context

Providing `context` helps the assistant give more relevant answers:
- If the user is viewing a specific team/app page, include that information
- The assistant can use this to provide more targeted help