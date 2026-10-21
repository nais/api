# nais.io users in tenant APIs

Set `ZITADEL_NAIS_ORGANIZATION_ID` (Helm: `zitadel.naisOrganizationID`) to the
nais.io organization ID to enable user sync and Console login. An empty value preserves
tenant-only login. Fasit supplies the value through `zitadel_nais_organization_id`.

The machine user configured by `ZITADEL_KEY` needs read access to users in the
nais.io organization, in addition to its existing tenant user-management access.
Terraform must also grant the nais.io organization access to each tenant's
Console project. These permissions must be deployed before enabling the setting.
The API also accepts CLI JWTs from the configured nais.io organization. Removing
the CLI's login restriction and adding tenant selection are handled separately.

## User identity

Email addresses are globally unique across tenants and Zitadel organizations:
the same email identifies the same user. Both user sources use the same
create/update logic, and Console login looks up both tenant and nais.io users by
email. The model does not distinguish separate identities with the same email.

The existing `external_id` field remains populated for compatibility with the
current tenant CLI and to preserve users when their email changes. It is not
used to distinguish nais.io users from tenant users during Console login.

## User lifecycle and permissions

- The API imports active human users from the configured Zitadel organization.
  Users are not copied into the tenant's Zitadel organization or modified there.
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

Console login no longer pins the flow to the tenant organization when nais.io
login is enabled. Users must still authenticate through the configured OIDC
provider and exist in the local API database.

CLI JWT authentication uses the same validation and user lookup for tenant and
nais.io users: the signature, issuer, audience and expiry must be valid, the
resource-owner organization must be either the tenant organization or the
configured nais.io organization, and the subject must match a synced user's
`external_id`. With no nais.io organization configured, only the tenant
organization is accepted. No token-format change is needed for this support.
Moving CLI authentication to email lookup later would require including email in
access tokens for both tenant and nais.io users.
