package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func runInternalHTTPServer(ctx context.Context, listenAddress string, reg prometheus.Gatherer, log logrus.FieldLogger) error {
	router := chi.NewRouter()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	router.Get("/healthz", func(_ http.ResponseWriter, _ *http.Request) {})

	router.HandleFunc("/pprof/*", pprof.Index)
	router.HandleFunc("/pprof/profile", pprof.Profile)
	router.HandleFunc("/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/pprof/trace", pprof.Trace)

	router.Handle("/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/pprof/mutex", pprof.Handler("mutex"))
	router.Handle("/pprof/heap", pprof.Handler("heap"))
	router.Handle("/pprof/block", pprof.Handler("block"))
	router.Handle("/pprof/allocs", pprof.Handler("allocs"))

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
		log.Infof("Internal HTTP server shutting down...")
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).Infof("HTTP server shutdown failed")
			return err
		}
		return nil
	})

	wg.Go(func() error {
		log.Infof("Internal HTTP server accepting requests on %q", listenAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Infof("unexpected error from HTTP server")
			return err
		}
		log.Infof("Internal HTTP server finished, terminating...")
		return nil
	})
	return wg.Wait()
}
