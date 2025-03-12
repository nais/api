package event

import (
	"context"
	"encoding/json"
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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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

	// State returns true when the watcher should be started/continue running and false when it should stop.
	state           []chan bool
	eventsCounter   metric.Int64Counter
	handlersCounter metric.Int64UpDownCounter
}

func NewWatcher(pool *pgxpool.Pool, clients map[string]kubernetes.Interface, log logrus.FieldLogger) (*Watcher, error) {
	chs := make([]chan bool, 0, len(clients))
	for range clients {
		chs = append(chs, make(chan bool, 1))
	}

	meter := otel.GetMeterProvider().Meter("nais_api_k8s_events")
	eventsCounter, err := meter.Int64Counter("nais_api_k8s_events_total", metric.WithDescription("Number of events processed"))
	if err != nil {
		return nil, fmt.Errorf("creating events counter: %w", err)
	}

	handlersCounter, err := meter.Int64UpDownCounter("nais_api_k8s_handlers", metric.WithDescription("number of goroutines handling events"))
	if err != nil {
		return nil, fmt.Errorf("creating handlers counter: %w", err)
	}

	return &Watcher{
		clients:         clients,
		events:          make(chan eventsql.UpsertParams, 20),
		queries:         eventsql.New(pool),
		log:             log,
		state:           chs,
		eventsCounter:   eventsCounter,
		handlersCounter: handlersCounter,
	}, nil
}

func (w *Watcher) Run(ctx context.Context) {
	w.wg = pool.New().WithErrors().WithContext(ctx)

	leaderelection.RegisterOnStartedLeading(w.onStartedLeading)
	leaderelection.RegisterOnStoppedLeading(w.onStoppedLeading)
	if leaderelection.IsLeader() {
		w.onStartedLeading(ctx)
	}

	w.wg.Go(func(ctx context.Context) error {
		return w.batchInsert(ctx)
	})

	i := 0
	for env, client := range w.clients {
		ch := w.state[i]
		i++
		w.wg.Go(func(ctx context.Context) error {
			return w.run(ctx, env, client, ch)
		})
	}

	if err := w.wg.Wait(); err != nil {
		w.log.WithError(err).Error("error running events watcher")
	}
}

func (w *Watcher) onStoppedLeading() {
	for _, ch := range w.state {
		select {
		case ch <- false:
		default:
			w.log.WithField("state", "stopped").Error("failed to send state")
		}
	}
}

func (w *Watcher) onStartedLeading(_ context.Context) {
	for _, ch := range w.state {
		select {
		case ch <- true:
		default:
			w.log.WithField("state", "started").Error("failed to send state")
		}
	}
}

var regHorizontalPodAutoscaler = regexp.MustCompile(`New size: (\d+); reason: (\w+).*(below|above) target`)

func (w *Watcher) run(ctx context.Context, env string, client kubernetes.Interface, state chan bool) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case s := <-state:
			w.log.WithField("env", env).WithField("state", s).Info("state change")
			if s {
				if err := w.watch(ctx, env, client, state); err != nil {
					w.log.WithError(err).Error("failed to watch events")
				}
				w.log.WithField("env", env).Info("stopped watching")

			}
		}
	}
}

func (w *Watcher) watch(ctx context.Context, env string, client kubernetes.Interface, state chan bool) error {
	w.handlersCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("environment", env)))
	defer w.handlersCounter.Add(ctx, -1, metric.WithAttributes(attribute.String("environment", env)))

	// Events we want to watch for
	// SuccessfulRescale - Check for successful rescale events
	// Killing - Check for liveness failures

	list, err := client.EventsV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	w.log.WithField("len", len(list.Items)).Debug("listed events")

	closeAndDrain := func(w watch.Interface) {
		w.Stop()
		for range w.ResultChan() {
			// Drain the channel
		}
	}

	rescale, err := client.EventsV1().Events("").Watch(ctx, metav1.ListOptions{
		FieldSelector: "reason=SuccessfulRescale,metadata.namespace!=nais-system",
	})
	if err != nil {
		return fmt.Errorf("failed to watch for rescale events: %w", err)
	}
	defer closeAndDrain(rescale)

	killing, err := client.EventsV1().Events("").Watch(ctx, metav1.ListOptions{
		FieldSelector: "reason=Killing,metadata.namespace!=nais-system",
	})
	if err != nil {
		return fmt.Errorf("failed to watch for killing events: %w", err)
	}
	defer closeAndDrain(killing)

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
		case s := <-state:
			if !s {
				return nil
			}
		case event := <-rescale.ResultChan():
			w.eventsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("environment", string(env)),
				attribute.String("type", string(event.Type)),
				attribute.String("reason", "SuccessfulRescale")),
			)

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
			w.eventsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("environment", string(env)),
				attribute.String("type", string(event.Type)),
				attribute.String("reason", "Killing")),
			)

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
