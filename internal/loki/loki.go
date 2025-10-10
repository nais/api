package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/sirupsen/logrus"
)

type Querier interface {
	// Tail returns a channel that will get log messages sent to it until the provided context is closed. The provided
	// filter is used to filter which log messages to receive.
	Tail(context.Context, *LogSubscriptionFilter) (<-chan *LogLine, error)
}

type querier struct {
	baseURL url.URL
	client  *http.Client
	logger  logrus.FieldLogger
}

func NewQuerier(lokiURL string, logger logrus.FieldLogger) (Querier, error) {
	client := &http.Client{}

	baseURLParsed, err := url.Parse(lokiURL)
	if err != nil {
		return nil, fmt.Errorf("parse loki URL: %w", err)
	}

	return &querier{
		client:  client,
		baseURL: *baseURLParsed,
		logger:  logger,
	}, nil
}

func (q *querier) Tail(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	u := q.baseURL
	u.Path = "/loki/api/v1/tail"

	since := -1 * time.Hour
	if filter.Since != nil {
		since = *filter.Since
	}

	params := u.Query()
	params.Set("query", filter.Query)

	if filter.Limit != nil {
		params.Set("limit", fmt.Sprintf("%d", *filter.Limit))
	}

	params.Set("start", fmt.Sprintf("%d", time.Now().Add(-since).UnixNano()))

	u.Scheme = "ws"
	u.RawQuery = params.Encode()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return nil, err
	}

	logLines := make(chan *LogLine, 1)

	go streamLogLines(ctx, conn, logLines, q.logger)

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
