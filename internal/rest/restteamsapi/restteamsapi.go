package restteamsapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/rest/restteamsapi/restteamsapisql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/sirupsen/logrus"
)

type Team struct {
	Members []string `json:"members"`
}

func TeamsApiHandler(ctx context.Context, pool *pgxpool.Pool, log logrus.FieldLogger) http.HandlerFunc {
	querier := restteamsapisql.New(pool)
	return func(rsp http.ResponseWriter, req *http.Request) {
		teamSlug := slug.Slug(req.PathValue("teamSlug"))

		err := teamSlug.Validate()
		if err != nil {
			log.Errorf("invalid team slug: %v", err)
			http.Error(rsp, err.Error(), http.StatusBadRequest)
			return
		}

		exists, err := querier.TeamExists(ctx, teamSlug)
		if err != nil {
			log.Errorf("failed to lookup team: %v", err)
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		if !exists {
			http.Error(rsp, team.ErrNotFound{}.Error(), http.StatusNotFound)
			return
		}

		members, err := querier.ListMembers(ctx, teamSlug)
		if err != nil {
			log.Errorf("failed to list team members: %v", err)
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}
		t := Team{
			Members: members,
		}

		enc, err := json.Marshal(t)
		if err != nil {
			log.Errorf("failed to marshall response: %v", err)
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}

		rsp.Header().Set("Content-Type", "application/json")
		rsp.WriteHeader(http.StatusOK)
		_, err = rsp.Write(enc)
		if err != nil {
			log.Errorf("error while writing response: %v", err)
			http.Error(rsp, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
