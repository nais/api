package rest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/rest/restteamsapi"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func Run(ctx context.Context, listenAddress string, pool *pgxpool.Pool, preSharedKey string, log logrus.FieldLogger) error {
	router := MakeRouter(ctx, pool, log, middleware.PreSharedKeyAuthentication(preSharedKey))

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

func MakeRouter(ctx context.Context, pool *pgxpool.Pool, log logrus.FieldLogger, middlewares ...func(http.Handler) http.Handler) *chi.Mux {
	router := chi.NewRouter()

	router.Use(middlewares...)

	router.Get("/teams/{teamSlug}", restteamsapi.TeamsApiHandler(ctx, pool, log))

	return router
}
