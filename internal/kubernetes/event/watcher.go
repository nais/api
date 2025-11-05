package event

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
	"github.com/nais/api/internal/kubernetes/event/pubsublog"
	"github.com/nais/api/internal/kubernetes/watcher"
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
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

type Watcher struct {
	queries        eventsql.Querier
	clients        map[string]kubernetes.Interface
	events         chan eventsql.UpsertParams
	log            logrus.FieldLogger
	logsSubscriber *pubsublog.Subscriber
	wg             *pool.ContextPool

	watchCtx             context.Context
	cancelWatchers       context.CancelFunc
	eventsCounter        metric.Int64Counter
	handlersCounter      metric.Int64UpDownCounter
	droppedEventsCounter metric.Int64Counter
}

func NewWatcher(pool *pgxpool.Pool, logsSubscription *pubsub.Subscription, clients map[string]kubernetes.Interface, restMappers map[string]watcher.KindResolver, log logrus.FieldLogger) (*Watcher, error) {
	meter := otel.GetMeterProvider().Meter("nais_api_k8s_events")
	eventsCounter, err := meter.Int64Counter("nais_api_k8s_events_total", metric.WithDescription("Number of events processed"))
	if err != nil {
		return nil, fmt.Errorf("creating events counter: %w", err)
	}

	handlersCounter, err := meter.Int64UpDownCounter("nais_api_k8s_handlers", metric.WithDescription("number of goroutines handling events"))
	if err != nil {
		return nil, fmt.Errorf("creating handlers counter: %w", err)
	}

	droppedEventsCounter, err := meter.Int64Counter("nais_api_k8s_events_dropped", metric.WithDescription("Number of events dropped due to channel overflow"))
	if err != nil {
		return nil, fmt.Errorf("creating dropped events counter: %w", err)
	}

	queries := eventsql.New(pool)

	logSub, err := pubsublog.NewSubscriber(logsSubscription, queries, restMappers, log.WithField("sub", "pubsub_log"))
	if err != nil {
		return nil, fmt.Errorf("creating pubsub log subscriber: %w", err)
	}

	return &Watcher{
		clients:              clients,
		events:               make(chan eventsql.UpsertParams, 1000),
		queries:              queries,
		log:                  log,
		eventsCounter:        eventsCounter,
		handlersCounter:      handlersCounter,
		droppedEventsCounter: droppedEventsCounter,
		logsSubscriber:       logSub,
	}, nil
}

func (w *Watcher) Run(ctx context.Context) {
	w.wg = pool.New().WithErrors().WithContext(ctx)

	leaderelection.RegisterOnStartedLeading(w.onStartedLeading)
	leaderelection.RegisterOnStoppedLeading(w.onStoppedLeading)

	w.wg.Go(func(ctx context.Context) error {
		return w.batchInsert(ctx)
	})

	w.wg.Go(w.logsSubscriber.Start)

	if leaderelection.IsLeader() {
		w.onStartedLeading(ctx)
	}

	if err := w.wg.Wait(); err != nil {
		w.log.WithError(err).Error("error running events watcher")
	}
}

func (w *Watcher) onStoppedLeading() {
	w.log.Info("leadership lost, stopping watchers")
	if w.cancelWatchers != nil {
		w.cancelWatchers()
		w.cancelWatchers = nil
	}
}

func (w *Watcher) onStartedLeading(ctx context.Context) {
	w.log.Info("leadership gained, starting watchers")

	// Cancel any existing watchers first
	if w.cancelWatchers != nil {
		w.cancelWatchers()
		// Wait a moment for old watchers to fully stop to avoid race conditions
		time.Sleep(100 * time.Millisecond)
	}

	// Create a new context for this leadership session
	w.watchCtx, w.cancelWatchers = context.WithCancel(ctx)

	// Start a watcher for each environment
	for env, client := range w.clients {
		env := env       // Capture loop variable
		client := client // Capture loop variable
		w.wg.Go(func(ctx context.Context) error {
			return w.runWithRetry(w.watchCtx, env, client)
		})
	}
}

var regHorizontalPodAutoscaler = regexp.MustCompile(`New size: (\d+); reason: (\w+).*(below|above) target`)

func (w *Watcher) runWithRetry(ctx context.Context, env string, client kubernetes.Interface) error {
	w.log.WithField("env", env).Info("starting watch with retry")

	for {
		select {
		case <-ctx.Done():
			w.log.WithField("env", env).Info("context cancelled, stopping")
			return nil
		default:
		}

		err := w.watch(ctx, env, client)
		if err != nil {
			w.log.WithField("env", env).WithError(err).Error("watch failed, retrying in 5 seconds")
		} else {
			w.log.WithField("env", env).Info("watch stopped")
		}

		// If context is done, exit immediately
		if ctx.Err() != nil {
			return nil
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func (w *Watcher) watch(ctx context.Context, env string, client kubernetes.Interface) error {
	w.handlersCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("environment", env)))
	defer w.handlersCounter.Add(ctx, -1, metric.WithAttributes(attribute.String("environment", env)))

	newWatcher := func(selector string) (*watchtools.RetryWatcher, error) {
		list, err := client.EventsV1().Events("").List(ctx, metav1.ListOptions{
			FieldSelector: selector,
			Limit:         10,
		})
		if err != nil {
			return nil, err
		}

		return watchtools.NewRetryWatcher(list.ResourceVersion, &cache.ListWatch{
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = selector
				return client.EventsV1().Events("").Watch(ctx, options)
			},
		})
	}

	// Events we want to watch for
	// SuccessfulRescale - Check for successful rescale events
	// Killing - Check for liveness failures
	closeAndDrain := func(w watch.Interface) {
		w.Stop()
		for range w.ResultChan() {
			// Drain the channel
		}
	}

	rescale, err := newWatcher("reason=SuccessfulRescale,metadata.namespace!=nais-system")
	if err != nil {
		return fmt.Errorf("failed to watch for rescale events: %w", err)
	}
	defer closeAndDrain(rescale)

	killing, err := newWatcher("reason=Killing,metadata.namespace!=nais-system")
	if err != nil {
		return fmt.Errorf("failed to watch for killing events: %w", err)
	}
	defer closeAndDrain(killing)

	handleEvent := func(event watch.Event, convert func(e *eventv1.Event) (eventsql.UpsertParams, bool)) {
		if event.Type != watch.Added && event.Type != watch.Modified {
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

		// Try to send event, but don't block if channel is full
		select {
		case w.events <- e:
		default:
			w.log.WithField("env", env).Warn("events channel full, dropping event")
			w.droppedEventsCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("environment", env)))
		}
	}

	w.log.WithField("env", env).Debug("watching events")
	for {
		select {
		case <-ctx.Done():
			w.log.WithField("env", env).Info("context cancelled, stopping watch")
			return nil
		case event, ok := <-rescale.ResultChan():
			if !ok {
				w.log.WithField("env", env).WithField("watcher", "rescale").Error("watching events returned closed channel")
				return fmt.Errorf("rescale watcher channel closed")
			}
			w.eventsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("environment", string(env)),
				attribute.String("type", string(event.Type)),
				attribute.String("reason", "SuccessfulRescale")),
			)

			handleEvent(event, func(e *eventv1.Event) (eventsql.UpsertParams, bool) {
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
		case event, ok := <-killing.ResultChan():
			if !ok {
				w.log.WithField("env", env).WithField("watcher", "killing").Error("watching events returned closed channel")
				return fmt.Errorf("killing watcher channel closed")
			}

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

	flush := func() {
		if len(events) > 0 {
			w.log.WithField("count", len(events)).Debug("flushing events")
			w.queries.Upsert(ctx, events).Exec(func(i int, err error) {
				if err != nil {
					w.log.WithError(err).Error("failed to insert event")
				}
			})
			events = nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			// Flush any remaining events before shutting down
			flush()
			return nil
		case event := <-w.events:
			events = append(events, event)
			// Flush immediately if we have a large batch
			if len(events) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
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

	ts := lastEventTimestamp(e)
	if ts.Time.IsZero() {
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

		TriggeredAt: ts,
	}, true
}

func lastEventTimestamp(e *eventv1.Event) pgtype.Timestamptz {
	if e.Series != nil && !e.Series.LastObservedTime.IsZero() {
		return pgtype.Timestamptz{
			Time:  e.Series.LastObservedTime.Time,
			Valid: true,
		}
	}

	if !e.DeprecatedLastTimestamp.IsZero() {
		return pgtype.Timestamptz{
			Time:  e.DeprecatedLastTimestamp.Time,
			Valid: true,
		}
	}

	if !e.CreationTimestamp.IsZero() {
		return pgtype.Timestamptz{
			Time:  e.CreationTimestamp.Time,
			Valid: true,
		}
	}

	return pgtype.Timestamptz{Valid: false}
}
