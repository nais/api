# nais.io users in tenant APIs

Set `ZITADEL_NAIS_ORGANIZATION_ID` (Helm: `zitadel.naisOrganizationID`) to the
nais.io organization ID to enable user sync and login. An empty value preserves
tenant-only login. Fasit supplies the value through `zitadel_nais_organization_id`.

The machine user configured by `ZITADEL_KEY` needs read access to users in the
nais.io organization, in addition to its existing tenant user-management access.
Terraform must also grant the nais.io organization access to each tenant's
Console project. These permissions must be deployed before enabling the setting.
The CLI's login restriction and tenant selection are handled separately.

## User lifecycle and permissions

- The API imports active human users from the configured Zitadel organization.
  Users are not copied into the tenant's Zitadel organization or modified there.
- The Zitadel user ID is stored as `external_id`, matching the token subject.
  Neither source adopts existing users by email from another identity. An email
  collision with a different external ID aborts the database transaction,
  preserving existing identities, roles and sessions. This also applies to
  identities absent from the current snapshots.
- New nais.io users have the existing authenticated-user read access (Viewer),
  without the tenant default `Team creator` role, admin access or team membership.
- Both user snapshots are fetched before database changes. Failure to fetch any
  page aborts synchronization without deleting database users.
- Users absent from the next complete snapshot, including inactive, locked and
  deleted Zitadel users, are deleted from the API database. Their sessions are
  deleted by the database's foreign-key cascade.
- Changes are applied on the existing usersync schedule (every 15 minutes).
  First-time users must exist in Zitadel and be synchronized before API login.

Zitadel is the source of truth for nais.io users. Suspending a user in Google
Workspace alone is not sufficient: deactivation must also reach Zitadel.

JWT authentication continues to check the issuer, audience and resource-owner
organization, accepting only the tenant organization and the configured nais.io
organization. The subject must match a local user. Console login uses the token
subject rather than email for Zitadel identities and no longer pins the login
flow to the tenant organization when nais.io login is enabled.
