package issue

import nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"

// Old issue snapshots key Postgres by its physical main identity. SQL only uses
// this alternative for stored POSTGRES_BRANCH rows; canonical rows and other
// resource kinds continue to match their logical resource name normally.
func legacyPostgresIssueName(name *string) *string {
	if name == nil {
		return nil
	}
	return new(nais_io_v1.PostgresPhysicalName(*name))
}
