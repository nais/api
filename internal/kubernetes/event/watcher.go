package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	eventsql "github.com/nais/api/internal/kubernetes/event/searchsql"
	"github.com/nais/api/internal/leaderelection"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	eventv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type Watcher struct {
	queries eventsql.Querier
	clients map[string]kubernetes.Interface
	events  chan eventsql.UpsertParams
	log     logrus.FieldLogger
	wg      *pool.ContextPool

	cancel context.CancelFunc
}

func NewWatcher(pool *pgxpool.Pool, clients map[string]kubernetes.Interface, log logrus.FieldLogger) *Watcher {
	return &Watcher{
		clients: clients,
		events:  make(chan eventsql.UpsertParams, 20),
		queries: eventsql.New(pool),
		log:     log,
	}
}

func (w *Watcher) Run(ctx context.Context) {
	w.wg = pool.New().WithContext(ctx)

	leaderelection.RegisterOnStartedLeading(w.onStartedLeading)
	leaderelection.RegisterOnStoppedLeading(w.onStoppedLeading)
	if leaderelection.IsLeader() {
		w.log.Debug("Is already leader, force start")
		w.onStartedLeading(ctx)
	}

	w.wg.Go(func(ctx context.Context) error {
		return w.batchInsert(ctx)
	})

	if err := w.wg.Wait(); err != nil {
		w.log.WithError(err).Error("error running events watcher")
	}
}

func (w *Watcher) onStoppedLeading() {
	w.log.Debug("onStoppedLeading...")
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil

		w.log.Debug("cancelling")
	}
}

func (w *Watcher) onStartedLeading(ctx context.Context) {
	if w.cancel != nil {
		w.cancel()
	}

	go func() {
		time.Sleep(5 * time.Second)
		w.onStoppedLeading()
	}()

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	for env, client := range w.clients {
		w.wg.Go(func(_ context.Context) error {
			w.log.WithField("env", env).Debug("starting watcher")
			return w.run(ctx, env, client)
		})
	}
}

var regHorizontalPodAutoscaler = regexp.MustCompile(`New size: (\d+); reason: (\w+).*(below|above) target`)

func (w *Watcher) run(ctx context.Context, env string, client kubernetes.Interface) error {
	for {
		if err := w.watch(ctx, env, client); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			w.log.WithError(err).Error("failed to watch events")
		}
		w.log.WithField("env", env).Info("stopped watching")
		time.Sleep(2 * time.Second)
	}
}

func (w *Watcher) watch(ctx context.Context, env string, client kubernetes.Interface) error {
	// Events we want to watch for
	// SuccessfulRescale - Check for successful rescale events
	// Killing - Check for liveness failures

	list, err := client.EventsV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	w.log.WithField("len", len(list.Items)).Debug("listed events")

	rescale, err := client.EventsV1().Events("").Watch(ctx, metav1.ListOptions{
		FieldSelector: "reason=SuccessfulRescale,metadata.namespace!=nais-system",
	})
	if err != nil {
		return fmt.Errorf("failed to watch for rescale events: %w", err)
	}
	defer rescale.Stop()

	killing, err := client.EventsV1().Events("").Watch(ctx, metav1.ListOptions{
		FieldSelector: "reason=Killing,metadata.namespace!=nais-system",
	})
	if err != nil {
		return fmt.Errorf("failed to watch for killing events: %w", err)
	}
	defer killing.Stop()

	handleEvent := func(event watch.Event, convert func(e *eventv1.Event) (eventsql.UpsertParams, bool)) {
		if event.Type != watch.Added && event.Type != watch.Modified {
			return
		}

		if !leaderelection.IsLeader() {
			return
		}

		v, ok := event.Object.(*eventv1.Event)
		if !ok {
			w.log.WithField("type", fmt.Sprintf("%T", event.Object)).Error("unexpected event type")
			return
		}

		e, ok := convert(v)
		if !ok {
			return
		}

		w.events <- e
	}

	w.log.WithField("env", env).Debug("watching events")
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-rescale.ResultChan():
			handleEvent(event, func(e *eventv1.Event) (eventsql.UpsertParams, bool) {
				if !strings.HasPrefix(e.Note, "New size") {
					w.log.WithField("note", e.Note).Debug("ignoring event")
				}

				matches := regHorizontalPodAutoscaler.FindStringSubmatch(e.Note)
				if len(matches) != 4 {
					return eventsql.UpsertParams{}, false
				}

				var direction string
				switch matches[3] {
				case "below":
					direction = "down"
				case "above":
					direction = "up"
				default:
					direction = "unknown"
				}

				data := map[string]any{
					"newSize":   matches[1],
					"direction": direction,
					"target":    matches[3],
				}

				return w.toUpsertParams(env, e, data)
			})
		case event := <-killing.ResultChan():
			handleEvent(event, func(e *eventv1.Event) (eventsql.UpsertParams, bool) {
				if strings.HasSuffix(e.Note, "failed liveness probe, will be restarted") {
					// Match `Container some-container-name failed liveness probe, will be restarted`
					data := map[string]any{
						"reason":    "liveness_probe_failed",
						"container": strings.SplitN(e.Note, " ", 3)[1],
					}
					return w.toUpsertParams(env, e, data)
				}

				return eventsql.UpsertParams{}, false
			})
		}
	}
}

func (w *Watcher) batchInsert(ctx context.Context) error {
	var events []eventsql.UpsertParams
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-w.events:
			events = append(events, event)
		case <-ticker.C:
			if len(events) > 0 {
				w.queries.Upsert(ctx, events).Exec(func(i int, err error) {
					if err != nil {
						w.log.WithError(err).Error("failed to insert event")
					}
				})
				events = nil
			}
		}
	}
}

func (w *Watcher) toUpsertParams(environmentName string, e *eventv1.Event, data map[string]any) (eventsql.UpsertParams, bool) {
	uid, err := uuid.Parse(string(e.GetUID()))
	if err != nil {
		w.log.WithError(err).Error("failed to parse event UID")
		return eventsql.UpsertParams{}, false
	}

	if len(data) == 0 {
		w.log.WithField("event", e).Debug("no data to insert")
		return eventsql.UpsertParams{}, false
	}

	b, err := json.Marshal(data)
	if err != nil {
		w.log.WithError(err).Error("failed to marshal event data")
		return eventsql.UpsertParams{}, false
	}

	return eventsql.UpsertParams{
		Uid:               uid,
		EnvironmentName:   environmentName,
		InvolvedKind:      e.Regarding.Kind,
		InvolvedName:      e.Regarding.Name,
		InvolvedNamespace: e.Regarding.Namespace,
		Data:              b,
		Reason:            e.Reason,

		TriggeredAt: pgtype.Timestamptz{
			Time:  e.CreationTimestamp.Time, // This should be the last time the event was updated/created
			Valid: !e.CreationTimestamp.IsZero(),
		},
	}, true
}
