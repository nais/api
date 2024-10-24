package fake

import (
	"context"
	"time"

	"github.com/nais/api/internal/v1/workload/podlog"
)

type streamer struct{}

func NewLogStreamer() podlog.Streamer {
	return &streamer{}
}

func (f *streamer) Logs(ctx context.Context, _ *podlog.WorkloadLogSubscriptionFilter) (<-chan *podlog.WorkloadLogLine, error) {
	ch := make(chan *podlog.WorkloadLogLine, 10)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ch <- &podlog.WorkloadLogLine{
					Time:     time.Now(),
					Message:  "Subscription closed.",
					Instance: "api",
				}
				close(ch)
				return
			case ch <- &podlog.WorkloadLogLine{
				Time:     time.Now(),
				Message:  "some message",  // TODO: Use "real" log messages
				Instance: "some instance", // TODO: Pick stuff from the team instead of a static instance?
			}:
				time.Sleep(1 * time.Second) // TODO: Configurable interval?
			}
		}
	}()
	return ch, nil
}
