package costupdater

import (
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/cost/costsql"
)

func TestPostgresCostImportIdentity(t *testing.T) {
	date := civil.Date{Year: 2026, Month: time.March, Day: 1}
	base := bigQueryCostTableRow{
		Env:      bigquery.NullString{StringVal: "dev", Valid: true},
		Team:     bigquery.NullString{StringVal: "team", Valid: true},
		App:      bigquery.NullString{StringVal: "orders", Valid: true},
		CostType: "Compute Engine", Date: date, Cost: 4,
	}
	for _, tt := range []struct {
		name         string
		modify       func(*bigQueryCostTableRow)
		wantPostgres *string
		wantApp      string
		wantError    bool
	}{
		{name: "legacy app without Postgres identifier", wantApp: "orders"},
		{name: "null application remains unowned", modify: func(row *bigQueryCostTableRow) { row.App = bigquery.NullString{} }},
		{name: "Postgres is separate from same-named app", modify: func(row *bigQueryCostTableRow) {
			row.PostgresName = bigquery.NullString{StringVal: "orders", Valid: true}
		}, wantPostgres: new("orders")},
		{name: "another Postgres is not merged", modify: func(row *bigQueryCostTableRow) {
			row.PostgresName = bigquery.NullString{StringVal: "other", Valid: true}
		}, wantPostgres: new("other")},
		{name: "empty Postgres identity rejected", modify: func(row *bigQueryCostTableRow) {
			row.PostgresName = bigquery.NullString{Valid: true}
		}, wantError: true},
		{name: "missing environment rejected for Postgres", modify: func(row *bigQueryCostTableRow) {
			row.PostgresName = bigquery.NullString{StringVal: "orders", Valid: true}
			row.Env = bigquery.NullString{}
		}, wantError: true},
		{name: "missing team rejected for Postgres", modify: func(row *bigQueryCostTableRow) {
			row.PostgresName = bigquery.NullString{StringVal: "orders", Valid: true}
			row.Team = bigquery.NullString{}
		}, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := base
			if tt.modify != nil {
				tt.modify(&row)
			}
			got, err := row.upsertParams()
			if tt.wantError {
				if err == nil {
					t.Error("expected invalid Postgres identity to fail import")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := costsql.CostUpsertParams{
				Environment: new("dev"), TeamSlug: "team", AppLabel: tt.wantApp,
				PostgresName: tt.wantPostgres, Service: "Compute Engine",
				Date: pgtype.Date{Time: date.In(time.UTC), Valid: true}, DailyCost: 4,
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("import params (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPostgresImportPreservesServiceDimension(t *testing.T) {
	row := bigQueryCostTableRow{
		Env:          bigquery.NullString{StringVal: "prod", Valid: true},
		Team:         bigquery.NullString{StringVal: "team", Valid: true},
		PostgresName: bigquery.NullString{StringVal: "orders", Valid: true},
		ServiceName:  bigquery.NullString{StringVal: "storage-volume", Valid: true},
		CostType:     "Cloud Storage", Cost: 0,
		Date: civil.Date{Year: 2026, Month: time.March, Day: 1},
	}
	got, err := row.upsertParams()
	if err != nil {
		t.Fatal(err)
	}
	if got.Service != row.CostType || got.ServiceName == nil || *got.ServiceName != "storage-volume" || got.DailyCost != 0 {
		t.Errorf("service breakdown or explicit zero changed: %+v", got)
	}
}
