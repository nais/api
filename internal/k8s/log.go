package k8s

import (
	"bufio"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"
)

func (c *Client) LogStream(ctx context.Context, cluster, namespace, selector, container string, instances []string) (<-chan *model.LogLine, error) {
	pods, err := c.clientSets[cluster].CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}

	for _, p := range pods.Items {
		if p.Labels["logs.nais.io/flow-secure-logs"] == "true" {
			return nil, apierror.Errorf("Logs are secure, cannot be streamed")
		}
	}

	wg := &sync.WaitGroup{}
	ch := make(chan *model.LogLine, 10)
	for _, pod := range pods.Items {
		pod := pod
		if len(instances) > 0 && !slices.Contains(instances, pod.Name) {
			continue
		}
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			logs, err := c.clientSets[cluster].CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container:  container,
				Follow:     true,
				Timestamps: true,
				TailLines:  ptr.To[int64](int64(150 / len(pods.Items))),
			}).Stream(ctx)
			if err != nil {
				c.log.Error(err)
				return
			}
			defer logs.Close()
			sc := bufio.NewScanner(logs)

			for sc.Scan() {
				line := sc.Text()
				parts := strings.SplitN(line, " ", 2)
				if len(parts) != 2 {
					continue
				}
				time, err := time.Parse(time.RFC3339Nano, parts[0])
				if err != nil {
					continue
				}

				t := &model.LogLine{
					Time:     time,
					Message:  parts[1],
					Instance: pod.Name,
				}

				select {
				case <-ctx.Done():
					// Exit on cancellation
					c.log.Info("closing subscription")
					return

				case ch <- t:
					// Our message went through, do nothing
				}

			}

			c.log.Debugf("Logs done, err: %v", sc.Err())
		}(wg)
	}
	go func() {
		wg.Wait()
		c.log.Info("closing subscription with explicit message")
		ch <- &model.LogLine{
			Time:     time.Now(),
			Message:  "Subscription closed.",
			Instance: "api",
		}
		close(ch)
	}()
	return ch, nil
}

func (c *Client) Log(ctx context.Context, cluster, namespace, pod, container string, tailLines int64) ([]*model.LogLine, error) {
	logs, err := c.clientSets[cluster].CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		TailLines:  &tailLines,
		Container:  container,
		Follow:     false,
		Timestamps: true,
	}).Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer logs.Close()

	sc := bufio.NewScanner(logs)

	ret := []*model.LogLine{}

	for sc.Scan() {
		line := sc.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			continue
		}
		ret = append(ret, &model.LogLine{
			Time:    t,
			Message: parts[1],
		})
	}

	return ret, nil
}
