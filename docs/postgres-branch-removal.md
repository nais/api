# Postgres branch removal: compatibility release

This is the first compatibility step towards independent Postgres databases. It
does not migrate database ownership, remove CRDs, or change the GraphQL schema.

## Preserve newer operator fields

`updatePostgres` keeps the original Kubernetes resource and applies only fields
that actually changed through the API: user labels, high availability, individual
resource quantities and the requested extension list. Unknown spec fields,
unrequested resource settings, metadata and status are not reconstructed from the
API's pinned Go types. Resource version is retained; Kubernetes conflicts are
returned rather than retried with stale intent.

Deploy this release to every API replica before introducing new operator-owned
spec fields. An older API replica can still lose those fields during an update.

## Freeze the legacy branch lifecycle

Set the Helm/Fasit value `postgres.branchWritesDisabled: true`, or the environment
variable `POSTGRES_BRANCH_WRITES_DISABLED=true` when running the API directly.
The default is `false`, preserving existing behavior until the rollout is ready.

With the freeze enabled, `createPostgresBranch`, `activatePostgresBranch` and
`deletePostgresBranch` return a user-visible error before any Kubernetes access or
successful activity logging. Their schema and authorization checks are unchanged;
old browser tabs and CLI versions get an explicit rejection, not a successful
no-op or an operation on a different resource.

The freeze applies to all environments served by this API deployment. It does not
block database/branch reads, personal access, normal Postgres creation/updates or
whole-Postgres deletion (which retains its existing safety checks).

This is startup configuration, not an instantaneous distributed lock. Complete
the rollout to all replicas, let in-flight branch requests finish, and recheck
the main-only inventory before starting any ownership migration. Direct Kubernetes
writers and GitOps are not fenced by this API setting. A separate per-database
migration lock and the actual ownership-transfer protocol are later steps.

Keep the freeze enabled while branches are being removed. Disabling it only
restores the legacy API operations; it does not reverse any database migration.
Do not roll back to an older API after new operator fields have been persisted
without confirming that its writes preserve them.
