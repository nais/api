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

---

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
| `conversation_id` | string | No | UUID of an existing conversation to continue |
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
  "blocks": [
    {
      "type": "text",
      "text": "To deploy your application, you need to..."
    }
  ],
  "sources": [
    {
      "title": "Deploying Applications",
      "url": "https://docs.nais.io/deploy/"
    }
  ],
  "usage": {
    "input_tokens": 150,
    "output_tokens": 75,
    "total_tokens": 225
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `conversation_id` | string | UUID of the conversation (new or existing) |
| `message_id` | string | UUID for this specific response message |
| `content` | string | Plain text of the response (concatenation of all text blocks) |
| `blocks` | array | Ordered content blocks — see [Content Blocks](#content-blocks) |
| `sources` | array | Documentation sources retrieved for this query |
| `sources[].title` | string | Document title |
| `sources[].url` | string | Document URL |
| `usage.input_tokens` | int | Tokens in the request |
| `usage.output_tokens` | int | Tokens in the response |
| `usage.total_tokens` | int | Total tokens used |

---

### POST /agent/chat/stream

Streaming chat endpoint using Server-Sent Events (SSE). Sends a message and streams the response in real-time.

**Request Body:** Same as `POST /agent/chat`.

**Response:** `Content-Type: text/event-stream`

Each line is a Server-Sent Event in the format `data: <json>\n\n`. Parse each `data:` value as JSON and handle it by `type`.

#### Event Types

**`metadata`** — Sent first. Contains the conversation and message IDs for this turn.
```json
{
  "type": "metadata",
  "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
  "message_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
}
```

**`content`** — A chunk of the assistant's text response. Concatenate these in order to build the full response text.
```json
{
  "type": "content",
  "content": "To deploy"
}
```

**`thinking`** — The model's reasoning/thought process (only when thinking mode is enabled on the server). See [Thinking Mode](#thinking-mode).
```json
{
  "type": "thinking",
  "thinking": "The user is asking about deployment. I should look up their applications first."
}
```

**`tool_start`** — A tool call has begun. Use this to show a "working…" indicator in the UI.
```json
{
  "type": "tool_start",
  "tool_call_id": "call_123456",
  "tool_name": "execute_graphql",
  "description": "Executing execute_graphql..."
}
```

**`tool_end`** — A tool call has finished.
```json
{
  "type": "tool_end",
  "tool_call_id": "call_123456",
  "tool_name": "execute_graphql",
  "description": "Executed execute_graphql",
  "tool_success": true
}
```

**`chart`** — A Prometheus chart to be rendered by the client. See [Rendering Charts](#rendering-charts).
```json
{
  "type": "chart",
  "chart": {
    "chart_type": "line",
    "title": "CPU Usage for my-app",
    "environment": "dev",
    "query": "sum(rate(container_cpu_usage_seconds_total{app=\"my-app\"}[5m])) by (pod)",
    "interval": "1h",
    "y_format": "cpu_cores",
    "label_template": "{pod}"
  }
}
```

**`sources`** — Documentation sources retrieved for this query. Sent once near the end of the stream, before `done`. Only present if the RAG search returned results.
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

**`usage`** — Token usage for the full turn. Sent once near the end of the stream, before `done`.
```json
{
  "type": "usage",
  "usage": {
    "input_tokens": 150,
    "output_tokens": 75,
    "total_tokens": 225,
    "max_tokens": 131072
  }
}
```

**`done`** — The stream is complete. Stop processing after receiving this.
```json
{
  "type": "done"
}
```

**`error`** — A fatal error occurred. Stop processing after receiving this.
```json
{
  "type": "error",
  "error_code": "stream_error",
  "error_message": "Description of the error"
}
```

#### Event Order

A complete stream always follows this order:

1. `metadata` (always first)
2. Zero or more cycles of: `thinking`?, `tool_start`, `tool_end`, `chart`?
3. `thinking`? chunks (if the model reasons before replying)
4. `content` chunks (the actual response text)
5. `usage` (once, before done)
6. `sources`? (once, only if RAG returned results)
7. `done`

If an `error` event is received at any point, the stream ends immediately.

#### Example: Simple response

```
data: {"type":"metadata","conversation_id":"550e8400-e29b-41d4-a716-446655440000","message_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8"}

data: {"type":"tool_start","tool_call_id":"call_123","tool_name":"execute_graphql","description":"Executing execute_graphql..."}

data: {"type":"tool_end","tool_call_id":"call_123","tool_name":"execute_graphql","description":"Executed execute_graphql","tool_success":true}

data: {"type":"content","content":"Based on "}

data: {"type":"content","content":"the information I found, "}

data: {"type":"content","content":"your application is healthy."}

data: {"type":"usage","usage":{"input_tokens":150,"output_tokens":42,"total_tokens":192,"max_tokens":131072}}

data: {"type":"sources","sources":[{"title":"Deploying Applications","url":"https://docs.nais.io/deploy/"}]}

data: {"type":"done"}
```

#### Example: Response with thinking

When thinking mode is enabled, reasoning chunks arrive before and between tool calls:

```
data: {"type":"metadata","conversation_id":"550e8400-e29b-41d4-a716-446655440000","message_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8"}

data: {"type":"thinking","thinking":"The user wants to know about deployment. I should check their applications first."}

data: {"type":"tool_start","tool_call_id":"call_123","tool_name":"execute_graphql","description":"Executing execute_graphql..."}

data: {"type":"tool_end","tool_call_id":"call_123","tool_name":"execute_graphql","description":"Executed execute_graphql","tool_success":true}

data: {"type":"thinking","thinking":"I have the application list. Now I can give deployment instructions."}

data: {"type":"content","content":"To deploy your application, "}

data: {"type":"content","content":"push to the main branch and the pipeline will handle the rest."}

data: {"type":"usage","usage":{"input_tokens":200,"output_tokens":55,"total_tokens":255,"max_tokens":131072}}

data: {"type":"sources","sources":[{"title":"CI/CD Guide","url":"https://docs.nais.io/cicd/"}]}

data: {"type":"done"}
```

#### Example: Response with a chart

```
data: {"type":"metadata","conversation_id":"550e8400-e29b-41d4-a716-446655440000","message_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8"}

data: {"type":"tool_start","tool_call_id":"call_456","tool_name":"render_chart","description":"Executing render_chart..."}

data: {"type":"tool_end","tool_call_id":"call_456","tool_name":"render_chart","description":"Executed render_chart","tool_success":true}

data: {"type":"chart","chart":{"chart_type":"line","title":"CPU Usage for my-app","environment":"dev","query":"sum(rate(container_cpu_usage_seconds_total{app=\"my-app\"}[5m])) by (pod)","interval":"1h","y_format":"cpu_cores","label_template":"{pod}"}}

data: {"type":"content","content":"Here's your CPU usage over the last hour. "}

data: {"type":"content","content":"Usage looks stable around 0.5 cores."}

data: {"type":"usage","usage":{"input_tokens":180,"output_tokens":38,"total_tokens":218,"max_tokens":131072}}

data: {"type":"done"}
```

---

### GET /agent/conversations

List all conversations for the authenticated user, ordered by most recently updated.

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

Get a specific conversation with all its messages.

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
        "blocks": [
          {
            "type": "thinking",
            "thinking": "The user wants to deploy. Let me check their pipeline setup."
          },
          {
            "type": "text",
            "text": "To deploy your application..."
          }
        ],
        "sources": [
          {
            "title": "Deploying Applications",
            "url": "https://docs.nais.io/deploy/"
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

#### Content Blocks

Assistant messages contain a `blocks` array representing the content in the order it was produced. Render them top-to-bottom to reconstruct the full response.

**Block types:**

| Type | Fields | Description |
|------|--------|-------------|
| `thinking` | `thinking` | Model's reasoning/thought process |
| `text` | `text` | Regular text output |
| `chart` | `chart` | A Prometheus chart to render — same schema as the SSE `chart` event |

Tool calls and token usage are internal details and are not included in the blocks returned to clients.

**Example with all block types:**
```json
{
  "blocks": [
    {
      "type": "thinking",
      "thinking": "The user is asking about CPU usage. I should visualize this."
    },
    {
      "type": "text",
      "text": "Here's the CPU usage for your application:"
    },
    {
      "type": "chart",
      "chart": {
        "chart_type": "line",
        "title": "CPU Usage for my-app",
        "environment": "dev",
        "query": "sum(rate(container_cpu_usage_seconds_total{app=\"my-app\"}[5m])) by (pod)",
        "interval": "1h",
        "y_format": "cpu_cores",
        "label_template": "{pod}"
      }
    },
    {
      "type": "text",
      "text": "Usage has been stable around 0.5 cores."
    }
  ]
}
```

---

### DELETE /agent/conversations/{conversationID}

Delete a conversation and all its messages.

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
- `400 Bad Request` — Invalid request body or parameters
- `401 Unauthorized` — Authentication required
- `404 Not Found` — Conversation not found
- `500 Internal Server Error` — Server error

---

## Client Implementation Notes

### Handling SSE Streams

1. Set appropriate request headers:
   ```
   Accept: text/event-stream
   Cache-Control: no-cache
   ```
2. Parse each `data:` line as JSON.
3. Accumulate `content` events to build the full response text.
4. Use `tool_start` / `tool_end` events to show a progress indicator.
5. Render `chart` events inline as Prometheus chart components.
6. Optionally display `thinking` events as a collapsible reasoning section.
7. Stop processing on `done` or `error`.

### Building the Display from Stream Events

To reconstruct the same block layout as returned by `GET /conversations/{id}`, track state as events arrive:

1. On `thinking`: append a `thinking` block with the accumulated text.
2. On `tool_start` / `tool_end`: show a transient tool indicator (not stored as a block).
3. On `chart`: append a `chart` block.
4. On `content`: buffer the text; flush as a `text` block when the stream ends or a `chart` / `tool_start` interrupts it.
5. On `usage` / `sources` / `done`: finalise the display.

### Thinking Mode

When thinking mode is enabled on the server (`AGENT_VERTEX_AI_INCLUDE_THOUGHTS=true`), `thinking` events are streamed before the model's response. Thinking blocks are also persisted and returned by `GET /conversations/{id}`.

Consider:
- Showing a "thinking…" animation while `thinking` events are arriving.
- Rendering thinking blocks as a collapsible section in the conversation history.
- Providing a UI toggle so users can show or hide thinking blocks.

Thinking events are only emitted when using a Gemini model with thinking mode enabled. If not enabled, no `thinking` events are sent.

### Conversation Management

- A new conversation is created automatically when `conversation_id` is omitted.
- Conversation titles are derived from the first message.
- There is no hard cap on the number of conversations per user.

### Sources

Every response includes a `sources` array containing the documentation pages that were retrieved as context for the query (up to 5). These are always the pages the model had available — not a filtered subset — so the client can display all of them as "references used".

In the streaming endpoint, sources arrive as a single `sources` event just before `done`. In the non-streaming endpoint and in `GET /conversations/{id}`, they are included directly in the response / message object.

### Rendering Charts

When the model determines that metrics data is better visualised as a chart it emits a `chart` event (streaming) or a `chart` block (in stored conversations). Render a Prometheus chart component using the provided data.

#### Chart Schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `chart_type` | `"line"` | Yes | Chart type — only `line` is currently supported |
| `title` | string | Yes | Human-readable title |
| `environment` | string | Yes | Environment to query metrics from (e.g. `dev`, `prod`) |
| `query` | string | Yes | PromQL query to execute |
| `interval` | string | No | Time window: `1h`, `6h`, `1d`, `7d`, or `30d` (default: `1h`) |
| `y_format` | string | No | Y-axis format: `number`, `percentage`, `bytes`, `cpu_cores`, or `duration` |
| `label_template` | string | No | Series label template using `{label_name}` syntax, e.g. `{pod}` or `{pod}/{container}` |

#### Mapping to a chart component

```typescript
type PrometheusChartProps = {
  environmentName: string;    // chart.environment
  query: string;              // chart.query
  interval?: string;          // chart.interval (default "1h")
  labelFormatter: (labels: { name: string; value: string }[]) => string;
  formatYValue?: (value: number) => string;
};
```

#### Label Template Processing

```typescript
function createLabelFormatter(template: string | undefined) {
  if (!template) {
    return (labels: { name: string; value: string }[]) =>
      labels.map(l => l.value).join('/');
  }
  return (labels: { name: string; value: string }[]) => {
    let result = template;
    for (const label of labels) {
      result = result.replace(`{${label.name}}`, label.value);
    }
    return result;
  };
}
```

#### Y-Axis Format

| `y_format` | Description | Example |
|------------|-------------|---------|
| `number` | Plain number | `1,234.56` |
| `percentage` | Percentage | `45.2%` |
| `bytes` | Byte size | `1.5 GiB` |
| `cpu_cores` | CPU cores | `0.5 cores` |
| `duration` | Time duration | `2m 30s` |

#### Interval Values

| `interval` | Description |
|------------|-------------|
| `1h` | Last 1 hour |
| `6h` | Last 6 hours |
| `1d` | Last 1 day |
| `7d` | Last 7 days |
| `30d` | Last 30 days |