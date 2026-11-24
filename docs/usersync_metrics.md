# User sync monitoring

Nais API exposes these Prometheus metrics from the replica running user sync:

| Metric                                                                  | Meaning                                                                                                                                                                                                  |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `nais_usersync_runs_total{result="success\|error"}`                     | API database sync attempts by outcome.                                                                                                                                                                   |
| `nais_usersync_last_success_timestamp_seconds`                          | Unix timestamp of the last successful API sync. Not emitted before the first success.                                                                                                                    |
| `nais_usersync_duration_seconds`                                        | API sync duration histogram, excluding the independent Zitadel export.                                                                                                                                   |
| `nais_usersync_snapshot_users{source="google\|nais_io"}`                | User counts from the latest complete snapshots. The Google count is after group filtering. The `nais_io` series is emitted only if the import is enabled. A failed lookup leaves the last counts intact. |
| `nais_usersync_api_user_changes_total{action="create\|update\|delete"}` | Local user row changes, counted only after the database transaction commits. Role changes are not included.                                                                                              |
| `nais_usersync_zitadel_export_runs_total{result="success\|error"}`      | Exports of tenant users to Zitadel, tracked separately from the API sync.                                                                                                                                |

With Helm alerts enabled, the chart alerts after 45 minutes without a successful
API sync. Deletion counts are available as metrics but do not trigger an alert.

The `usersync complete` log includes the API sync outcome, source counts when
available, and committed create, update and delete counts. Use the admin-only
GraphQL `userSyncLog` query to identify affected users.
