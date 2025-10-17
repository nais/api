package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/sirupsen/logrus"
)

type Client interface {
	// Tail returns a channel that will get log messages sent to it until the provided context is closed. The provided
	// filter is used to filter which log messages to receive.
	Tail(context.Context, *LogSubscriptionFilter) (<-chan *LogLine, error)
}

func DefaultLokiUrlGenerator(clusterName, tenant string) (*url.URL, error) {
	if tenant == "nav" {
		clusterName = strings.TrimSuffix(clusterName, "-fss")
	}

	u, err := url.Parse(fmt.Sprintf("http://loki.%s.%s.cloud.nais.io", clusterName, tenant))
	if err != nil {
		return nil, fmt.Errorf("parse loki URL: %w", err)
	}
	return u, nil
}

type querier struct {
	// lokis is a map from environment names to Loki URLs
	lokis map[string]url.URL
	log   logrus.FieldLogger
}

type lokiUrlGeneratorFunc func(clusterName, tenant string) (*url.URL, error)

type setup struct {
	urlGenerator lokiUrlGeneratorFunc
}

type OptionFunc func(*setup)

func WithLocalLoki(addr string) OptionFunc {
	u, err := url.Parse(addr)
	return func(s *setup) {
		s.urlGenerator = func(string, string) (*url.URL, error) {
			return u, err
		}
	}
}

func NewClient(clusters []string, tenant string, log logrus.FieldLogger, opts ...OptionFunc) (Client, error) {
	s := &setup{}
	for _, opt := range opts {
		opt(s)
	}

	if s.urlGenerator == nil {
		s.urlGenerator = DefaultLokiUrlGenerator
	}

	lokis := make(map[string]url.URL, len(clusters))
	for _, cluster := range clusters {
		u, err := s.urlGenerator(cluster, tenant)
		if err != nil {
			return nil, fmt.Errorf("unable to generate Loki URL for cluster %q and tenant %q: %v", cluster, tenant, err)
		}

		lokis[environmentmapper.EnvironmentName(cluster)] = *u
	}

	return &querier{
		lokis: lokis,
		log:   log,
	}, nil
}

func (q *querier) Tail(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	lokiURL, ok := q.lokis[filter.EnvironmentName]
	if !ok {
		return nil, fmt.Errorf("unable to select Loki for cluster %q", filter.EnvironmentName)
	}

	lokiURL.Scheme = "wss"
	if strings.HasPrefix(lokiURL.Host, "localhost") || strings.HasPrefix(lokiURL.Host, "127.0.0.1") {
		lokiURL.Scheme = "ws"
	}

	lokiURL.Path = "/loki/api/v1/tail"
	lokiURL.RawQuery = filter.lokiQueryParameters().Encode()

	conn, err := connect(ctx, lokiURL, q.log)
	if err != nil {
		q.log.
			WithError(err).
			WithField("lokiUrl", lokiURL.String()).
			WithField("filter", filter).
			Error("unable to connect to Loki")
		return nil, err
	}

	logLines := make(chan *LogLine, 1)

	go func() {
		if err := streamLogLines(ctx, conn, logLines); err != nil {
			q.log.WithError(err).Errorf("streaming log lines")
			logLines <- &LogLine{
				Time:    time.Now(),
				Message: fmt.Sprintf("error streaming log lines: %v", err),
			}
		}
		close(logLines)
	}()

	return logLines, nil
}

func connect(ctx context.Context, u url.URL, log logrus.FieldLogger) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("connect to loki: %w", err)
	}

	go func() {
		<-ctx.Done()
		log.Debugf("closing log streamer connection")
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}()

	return conn, nil
}

func streamLogLines(ctx context.Context, conn *websocket.Conn, logLines chan *LogLine) error {
	for ctx.Err() == nil {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				return fmt.Errorf("read log message from loki: %w", err)
			}
			return nil
		}

		var resp loghttp.TailResponse
		if err := json.NewDecoder(bytes.NewReader(message)).Decode(&resp); err != nil {
			return fmt.Errorf("parse log message from loki: %w", err)
		}

		for _, stream := range resp.Streams {
			labels := make([]*LogLineLabel, 0)
			for k, v := range stream.Labels {
				labels = append(labels, &LogLineLabel{
					Key:   k,
					Value: v,
				})
			}

			for _, entry := range stream.Entries {
				logLines <- &LogLine{
					Time:    entry.Timestamp,
					Message: entry.Line,
					Labels:  labels,
				}
			}
		}
	}

	return nil
}
