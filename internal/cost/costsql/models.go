// Code generated by sqlc. DO NOT EDIT.

package costsql

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/slug"
)

type CostMonthlyTeam struct {
	TeamSlug         slug.Slug
	Month            pgtype.Date
	LastRecordedDate pgtype.Date
	DailyCost        float32
}

type CostMonthlyTenant struct {
	Service          string
	Month            pgtype.Date
	LastRecordedDate pgtype.Date
	DailyCost        float32
}
