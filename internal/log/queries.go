package log

import (
	"context"
	"fmt"
	"time"
)

func LogStream(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	c := make(chan *LogLine, 1)
	go func() {
		defer close(c)
		for i := range 100 {
			select {
			case <-ctx.Done():
				return
			case c <- &LogLine{Message: fmt.Sprintf("Hello, World! %d", i)}:
				time.Sleep(1 * time.Second)
			}
		}
	}()

	return c, nil
}
