# Audit Events

An audit event is a record of an event:

> Actor X performed action Y on resource Z with name A belonging to team B at time T.

## Anatomy of an Audit Event

An audit event consists of the following fields in the database:

| Field         | Type      | Description                                                                  |
| ------------- | --------- | ---------------------------------------------------------------------------- |
| id            | uuid      | A unique identifier.                                                         |
| actor         | text      | The user or service account that performed the event.                        |
| action        | text      | The action performed.                                                        |
| resource_type | text      | The type of resource the action affects.                                     |
| resource_name | text      | The name of the affected resource.                                           |
| created_at    | timestamp | The time of the event.                                                       |
| team          | slug      | Optional. The slug associated with the team that owns the affected resource. |
| env           | text      | Optional. The name of the environment associated with the affected resource. |
| data          | bytea     | Optional. Opaque blob of additional data associated with a concrete event.   |

## Conventions

All types and enums for audit events are defined in the GraphQL schema:

- [internal/graph/graphqls/auditevents.graphql](../internal/graph/graphqls/auditevents.graphqls)

### Resource types

A resource type is a logical grouping of a resource that we expose via the Graph e.g.:

```graphql
enum AuditEventResourceType {
	TEAM
	TEAM_MEMBER
}
```

### Actions

An action is a specific operation that can be performed on a resource, e.g:

```graphql
enum AuditEventAction {
	ADDED
	CREATED
	DELETED
	REMOVED
	RESTARTED
	UPDATED
}
```

Actions should be in the past tense.

An action may be prefixed with a sub-resource type that is too small to be its own resource type.

`<SubresourceType>_<Action>`

For example:

```graphql
enum AuditEventAction {
	TEAM_DELETION_CONFIRMED
	TEAM_DELETION_REQUESTED
	TEAM_DEPLOY_KEY_ROTATED
	TEAM_SET_PURPOSE
	TEAM_SET_DEFAULT_SLACK_CHANNEL
	TEAM_SET_ALERTS_SLACK_CHANNEL

	TEAM_MEMBER_SET_ROLE
}
```

### Event types

There are two types of events:

1. `BaseAuditEvent`: basic event that does not have any additional data.
2. `<ResourceType><Action>`: a concrete event that has additional data.

All events must implement the `AuditEvent` interface.

A concrete event with additional data needs its own type definition.
It must contain the `data` field with a concrete type that describes the additional data:

```graphql
type AuditEventMemberAdded implements AuditEvent {
	id: ID!
	action: AuditEventAction!
	actor: String!
	createdAt: Time!
	message: String!
	resourceType: AuditEventResourceType!
	resourceName: String!
	team: Slug!

	data: AuditEventMemberAddedData!
}

type AuditEventMemberAddedData {
	memberEmail: String!
	role: TeamRole!
}
```

The event must also be added to the union type `AuditEventNode` that is used to return a list of events.

### Go models

The Go models for audit events are generated from the GraphQL schema:

```shell
make generate-graphql
```

Structure:

```text
internal/
├─ audit/
│  ├─ model.go
├─ graph/
│  ├─ model/
│  │  ├─ model_gen.go
```

- [internal/graph/model/models_gen.go](../internal/graph/model/models_gen.go) - gqlgen generated models
- [internal/audit/model.go](../internal/audit/model.go) - audit event models that override the gqlgen generated models

All concrete event models with additional data must implement the `AuditEvent` interface.
This is done by embedding the `BaseAuditEvent` struct together with the associated data type and overriding the `GetData() any` member function:

```go
type AuditEventTeamSetAlertsSlackChannel struct {
	BaseAuditEvent
	Data model.AuditEventTeamSetAlertsSlackChannelData
}

func (a AuditEventTeamSetAlertsSlackChannel) GetData() any {
	return a.Data
}
```

## Defining new audit events

### Update Graph definition

[internal/graph/graphqls/auditevents.graphql](../internal/graph/graphqls/auditevents.graphqls)

1. If adding events to a new resource type, add the new type to the `AuditEventResourceType` enum.
1. Add the new action value to the `AuditEventAction` enum.
1. If the new event has associated data, create a new type definition for the data following the [Event types convention](#event-types).

### Update Go models

1. If the new event has associated data, define the new event type in the Go models following the [Go models convention](#go-models).
1. Generate the Go models from the GraphQL schema:

```shell
make generate-graphql
```

### Add new method to the Auditor for persisting the event

[internal/audit/auditor.go](../internal/audit/auditor.go)

### Add new branch(es) to the Graph resolver

[internal/audit/graph.go](../internal/audit/graph.go)

## Storing audit events

Audit events are stored to the database.

Use the `Auditor` instance [internal/audit/auditor.go](../internal/audit/auditor.go) to store events:

```go
err := r.auditor.TeamMemberAdded(ctx, actor.User, slug, user.Email, member.Role)
if err != nil {
    return nil, err
}
```

## Returning audit events

Audit events are fetched from the database and returned through the Graph.
The logic for mapping the database model to the Graph model located in [internal/audit/graph.go](../internal/audit/graph.go).
