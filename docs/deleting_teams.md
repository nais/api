# Deleting teams in Console

Deleting teams in Console is a complex operation, and requires a more thorough explanation.

Once a team is created, it will get an entry in the `teams` table. It will also have its slug added to the `team_slugs`
table. This is done using an `AFTER INSERT` trigger on the `teams` table. We also have a `BEFORE INSERT` trigger on the 
`teams` table, making sure a previoulsy registered team slug is not reusable.

Using this approach, we are able to retain the uniqueness of the team slug, even after the team is deleted, and we also 
get the automatic cleanup of related team-data using foreign keys and cascading deletes.

If we had used a more regular "soft delete" approach (for instance by adding a `deleted_at` column to the `teams` 
table), we would have to manually clean up the related data once a team had been deleted. Another issue with this 
approach is that most team-related queries would have to consider the `deleted_at` column when fetching teams from the 
database.

## How to perform a team deletion

Deleting a team from Console can not be done by a single individual. The process requires two separate team owners that 
have to perform the following operations:

1) Owner A creates a team deletion request by using the Console UI. This operation generates a URL that must be shared
with owner B.
2) Owner B visits the URL generated by owner A, and confirms the delete request.

Once a delete request is confirmed it **cannot** be aborted.

Since a team has many external resources (GitHub team, GCP project, CDN bucket, ...) the deletion process is 
asynchronous. If the process completes successfully, the team entry will be removed from the `teams` table, and it 
**will no longer** be accessible through the GraphQL API. 

If, on the other hand, the deletion process fails, the team will **not** be removed from the `teams` table, and errors 
during the deletion process can be fetched from the GraphQL API for further investigation and possible manual cleanup.

The actual deletion of external resources is handled by [api-reconcilers](https://github.com/nais/api-reconcilers) 
(which is also responsible for creating the external resources when the team was initially created).

The deletion process is triggered via a message queue.

## Reusing a team slug

As mentioned above, team slugs **cannot** be reused. This is by design, and prevents accidental takeover of potential 
external resources that the previous version of the team had access to.

## Restoring deleted teams

Restoring a deleted team is **not** supported. This is by design, and is related to the fact that a team
in Console has many external resources that might not be restorable using the same team slug (since the team slug is in
almost all cases used when creating external resources).