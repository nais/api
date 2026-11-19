package postgres

import "context"

// PhysicalStatus reads only Postgres status. Missing physical status means the
// database is still provisioning.
func PhysicalStatus(ctx context.Context, pg *Postgres) (PostgresState, *PostgresConfiguration, error) {
	if pg.directClusterName != nil {
		return pg.physicalState, pg.configuration, nil
	}
	return PostgresStateProgressing, nil, nil
}

func physicalClusterName(ctx context.Context, pg *Postgres) (*string, error) {
	return pg.directClusterName, nil
}
