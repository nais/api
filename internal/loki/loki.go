package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/sirupsen/logrus"
)

type Client interface {
	// Tail returns a channel that will get log messages sent to it until the provided context is closed. The provided
	// filter is used to filter which log messages to receive.
	Tail(context.Context, *LogSubscriptionFilter) (<-chan *LogLine, error)
}

type querier struct {
	// lokis is a map from cluster names to Loki URLs
	lokis map[string]url.URL
	log   logrus.FieldLogger
}

type lokiUrlGeneratorFunc func(cluster, tenant string) (*url.URL, error)

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
		s.urlGenerator = func(cluster, tenant string) (*url.URL, error) {
			u, err := url.Parse(fmt.Sprintf("http://loki.%s.%s.cloud.nais.io", cluster, tenant))
			if err != nil {
				return nil, fmt.Errorf("parse loki URL: %w", err)
			}
			return u, nil
		}
	}

	lokis := make(map[string]url.URL, len(clusters))
	for _, cluster := range clusters {
		u, err := s.urlGenerator(cluster, tenant)
		if err != nil {
			return nil, fmt.Errorf("unable to generate Loki URL for cluster %q and tenant %q: %v", cluster, tenant, err)
		}

		lokis[cluster] = *u
	}

	return &querier{
		lokis: lokis,
		log:   log,
	}, nil
}

func (q *querier) Tail(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	lokiUrl, ok := q.lokis[filter.EnvironmentName]
	if !ok {
		return nil, fmt.Errorf("unable to select Loki for cluster %q", filter.EnvironmentName)
	}

	lokiUrl.Scheme = "ws"
	lokiUrl.Path = "/loki/api/v1/tail"
	lokiUrl.RawQuery = filter.lokiQueryParameters().Encode()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, lokiUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	logLines := make(chan *LogLine, 1)

	go streamLogLines(ctx, conn, logLines, q.log)

	return logLines, nil
}

func streamLogLines(ctx context.Context, conn *websocket.Conn, logLines chan *LogLine, log logrus.FieldLogger) {
	defer func() {
		log.Debugf("closing log streamer connection")
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
		close(logLines)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer cancel()
		for ctx.Err() == nil {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.WithError(err).Errorf("read log message from loki")
				return
			}

			var resp loghttp.TailResponse
			if err := json.NewDecoder(bytes.NewReader(message)).Decode(&resp); err != nil {
				log.WithError(err).Errorf("parse log message from loki")
				return
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
	}()

	<-ctx.Done()
}
