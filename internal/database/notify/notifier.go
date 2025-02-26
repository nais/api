package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type Operation string

const (
	Insert Operation = "INSERT"
	Update Operation = "UPDATE"
	Delete Operation = "DELETE"
)

// Payload is the payload of a notification
// You should not change this struct, as it's used by all listeners
type Payload struct {
	Table string         `json:"table"`
	Op    Operation      `json:"op"`
	Data  map[string]any `json:"data"`
}

type listener struct {
	ch chan Payload
}

type Option func(n *Notifier)

func WithRetries(num int) Option {
	return func(n *Notifier) {
		n.maxRetries = num
	}
}

type Notifier struct {
	db         *pgxpool.Pool
	log        logrus.FieldLogger
	channel    string
	maxRetries int

	lock      sync.RWMutex
	listeners map[string][]listener
}

func New(db *pgxpool.Pool, log logrus.FieldLogger, opts ...Option) *Notifier {
	n := &Notifier{
		db:         db,
		channel:    "api_notify",
		log:        log,
		listeners:  map[string][]listener{},
		maxRetries: 100,
	}

	for _, opt := range opts {
		opt(n)
	}

	return n
}

func (n *Notifier) Run(ctx context.Context) {
	retries := 0
	lastError := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := n.run(ctx); err != nil {
				n.log.WithError(err).Error("error running notifier")
				if n.maxRetries == 0 || retries >= n.maxRetries {
					n.log.Errorf("max retries reached, shutting down notifier")
					return
				}
				if time.Since(lastError) > time.Minute {
					retries = 0
				}

				retries++
				lastError = time.Now()

				time.Sleep(time.Duration(retries) * time.Second)
			}
		}
	}
}

// SetChannel sets the channel to listen on
// Will not take effect after Run has been called
func (n *Notifier) SetChannel(channel string) {
	n.channel = channel
}

// Listen returns a channel that will receive notifications for the given table
// It will receive all notifications for the table unless one or more filters are provided
func (n *Notifier) Listen(table string) <-chan Payload {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.log.WithField("table", table).Debug("registering listener")

	ch := make(chan Payload, 20)
	n.listeners[table] = append(n.listeners[table], listener{
		ch: ch,
	})

	return ch
}

func (n *Notifier) run(ctx context.Context) error {
	conn, err := n.db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+n.channel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	for {
		not, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			switch {
			case errors.Is(err, pgx.ErrTxClosed):
				return nil
			case errors.Is(err, io.ErrUnexpectedEOF):
				n.log.WithError(err).Infof("listener got unexpected EOF, retry")
				continue
			}

			return fmt.Errorf("wait for notification: %w", err)
		}

		payload := Payload{}
		if err := json.Unmarshal([]byte(not.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}

		go n.distibute(payload)
	}
}

func (n *Notifier) distibute(payload Payload) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	n.log.WithFields(logrus.Fields{
		"table": payload.Table,
		"op":    payload.Op,
		"data":  payload.Data,
	}).Debug("received notification")

	listeners, ok := n.listeners[payload.Table]
	if !ok {
		return
	}

	for _, listener := range listeners {
		select {
		case listener.ch <- payload:
		default:
			n.log.WithField("table", payload.Table).Warn("listener channel full, dropping notification")
		}
	}
}
