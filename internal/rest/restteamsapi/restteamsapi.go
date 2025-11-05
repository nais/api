package restteamsapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/rest/resterror"
	"github.com/nais/api/internal/rest/restteamsapi/restteamsapisql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/sirupsen/logrus"
)

type Team struct {
	Members []string `json:"member"`
}

func TeamsApiHandler(ctx context.Context, pool *pgxpool.Pool, log logrus.FieldLogger) http.HandlerFunc {
	querier := restteamsapisql.New(pool)
	return func(rsp http.ResponseWriter, req *http.Request) {
		teamSlug := slug.Slug(req.PathValue("teamSlug"))

		err := teamSlug.Validate()
		if err != nil {
			log.Errorf("invalid team slug: %v", err)
			restErr := resterror.Wrap(http.StatusBadRequest, err)
			restErr.Write(rsp)
			return
		}

		exists, err := querier.TeamExists(ctx, teamSlug)
		if err != nil {
			log.Errorf("failed to lookup team: %v", err)
			restErr := resterror.Wrap(http.StatusInternalServerError, err)
			restErr.Write(rsp)
			return
		}

		if !exists {
			restErr := resterror.Wrap(http.StatusNotFound, team.ErrNotFound{})
			restErr.Write(rsp)
			return
		}

		members, err := querier.ListMembers(ctx, teamSlug)
		if err != nil {
			log.Errorf("failed to list team members: %v", err)
			restErr := resterror.Wrap(http.StatusInternalServerError, err)
			restErr.Write(rsp)
			return
		}
		t := Team{
			Members: members,
		}

		enc, err := json.Marshal(t)
		if err != nil {
			log.Errorf("failed to marshal response: %v", err)
			restErr := resterror.Wrap(http.StatusInternalServerError, err)
			restErr.Write(rsp)
			return
		}

		rsp.Header().Set("Content-Type", "application/json")
		rsp.WriteHeader(http.StatusOK)
		_, err = rsp.Write(enc)
		if err != nil {
			log.Errorf("error while writing response: %v", err)
			restErr := resterror.Wrap(http.StatusInternalServerError, err)
			restErr.Write(rsp)
			return
		}
	}
}
