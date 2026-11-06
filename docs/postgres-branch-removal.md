# Postgres field preservation and branch removal

## Scope of this API change

The current change only fixes field preservation in `updatePostgres`. It does not
remove branches, change the GraphQL schema, migrate database ownership or add a
rollout flag. Existing branch operations and personal access keep their current
behavior. This fix can be reviewed and deployed independently.

`updatePostgres` keeps the original Kubernetes resource and applies only fields
that actually changed through the API: user labels, high availability, individual
resource quantities and the requested extension list. Unknown spec fields,
unrequested resource settings, metadata and status are not reconstructed from the
API's pinned Go types. Resource version is retained; Kubernetes conflicts are
returned rather than retried with stale intent.

Deploy the fix to every API replica before relying on newer operator-owned spec
fields. It is useful independently of branch removal and does not itself require
an ownership migration or CLI update.

## Planned branch removal: coordinated maintenance window

The deployment has only a few test databases. Their data must be preserved, but
database management can stop during the change. We are therefore not building
old/new API projections, a virtual `main`, dual access forms or a long-running
compatibility rollout.

Build and test the branch-free pgrator/API/Console/CLI combination before the
window. Agree its shared resource contract, then publish the types/schema needed
to generate its clients. The complete branch-free API is a separate breaking
change; do not deploy it into the running branch-based stack on its own.

During the window:

1. Suspend Postgres management requests, new personal access and competing
   Kubernetes/GitOps writers. Wait for in-flight work and confirm access cleanup.
2. Stop old pgrator Postgres reconciliation before changing resource ownership.
3. Convert the test databases once, preserving Postgres and cluster identities,
   storage, poolers, roles/certificates, IAM and backup archives. Transfer both
   ownerReferences and ownership annotations; do not use normal branch deletion.
4. Install the new schemas/RBAC and start the new operator, then the branch-free
   API, updated workload integration and Console. Publish/use the matching CLI.
5. Verify the new combination and only then reopen Postgres management.

Relay keeps its existing transport and RelayAccess contract where the deployed
baseline is sufficient. Old Console tabs must reload and old CLI versions are not
supported after the window. Historic activity entries remain historic; their
schema/representation is not a live branch compatibility layer.

Remove the branch CRD only after verifying that no objects, controller owners,
finalizers or running consumers depend on it. Keep a metadata inventory and a
tested reverse-transfer/redeployment procedure: an old image alone is not a safe
rollback after ownership has changed. No live database operations are performed
by this API patch.

The earlier `postgres.branchWritesDisabled` / `POSTGRES_BRANCH_WRITES_DISABLED`
setting from the initial branch-removal draft has been removed. Do not rely on
that setting to establish a maintenance window.
