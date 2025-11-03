package teamsapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/teamsapi/teamsapisql"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Team struct {
	Members []string `json:"members"`
}

func Run(ctx context.Context, listenAddress string, pool *pgxpool.Pool, log logrus.FieldLogger) error {
	router := chi.NewRouter()
	router.Get("/teams/{teamSlug}", teamsApiHandler(ctx, pool, log))

	srv := &http.Server{
		Addr:              listenAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log.Infof("REST HTTP server shutting down...")
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).Infof("HTTP server shutdown failed")
			return err
		}
		return nil
	})

	wg.Go(func() error {
		log.Infof("REST HTTP server accepting requests on %q", listenAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Infof("unexpected error from HTTP server")
			return err
		}
		log.Infof("REST HTTP server finished, terminating...")
		return nil
	})
	return wg.Wait()
}

func teamsApiHandler(ctx context.Context, pool *pgxpool.Pool, log logrus.FieldLogger) http.HandlerFunc {
	querier := teamsapisql.New(pool)
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
