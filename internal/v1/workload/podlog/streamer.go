package podlog

import (
	"bufio"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nais/api/internal/v1/graphv1/apierror"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/ptr"
)

type Streamer interface {
	Logs(ctx context.Context, filter *WorkloadLogSubscriptionFilter) (<-chan *WorkloadLogLine, error)
}

type streamer struct {
	clientSets map[string]kubernetes.Interface
	log        logrus.FieldLogger
}

func NewLogStreamer(clientSets map[string]kubernetes.Interface, log logrus.FieldLogger) Streamer {
	return &streamer{
		clientSets: clientSets,
		log:        log,
	}
}

func (l *streamer) Logs(ctx context.Context, filter *WorkloadLogSubscriptionFilter) (<-chan *WorkloadLogLine, error) {
	k8sClientSet, exists := l.clientSets[filter.Environment]
	if !exists {
		return nil, apierror.Errorf("Environment %q does not exist.", filter.Environment)
	}

	coreV1Client := k8sClientSet.CoreV1()

	pods, err := getPods(ctx, coreV1Client, filter)
	if err != nil {
		return nil, err
	} else if len(pods) == 0 {
		return nil, apierror.Errorf("No pods found.")
	}

	namespace := filter.Team.String()
	container := ""
	switch {
	case filter.Application != nil:
		container = *filter.Application
	case filter.Job != nil:
		container = *filter.Job
	default:
		return nil, apierror.Errorf("No application or job specified in the filter.")
	}

	ch := make(chan *WorkloadLogLine, 10)
	wg := &sync.WaitGroup{}

	for _, pod := range pods {
		if len(filter.Instances) > 0 && !slices.Contains(filter.Instances, pod.Name) {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			logs, err := coreV1Client.Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container:  container,
				Follow:     true,
				Timestamps: true,
				TailLines:  ptr.To[int64](int64(150 / len(pods))),
			}).Stream(ctx)
			if err != nil {
				l.log.WithError(err).Errorf("getting logs")
				return
			}
			defer func() {
				if err := logs.Close(); err != nil {
					l.log.WithError(err).Errorf("closing logs")
				}
			}()

			sc := bufio.NewScanner(logs)

			for sc.Scan() {
				line := sc.Text()
				parts := strings.SplitN(line, " ", 2)
				if len(parts) != 2 {
					continue
				}
				ts, err := time.Parse(time.RFC3339Nano, parts[0])
				if err != nil {
					continue
				}

				entry := &WorkloadLogLine{
					Time:     ts,
					Message:  parts[1],
					Instance: pod.Name,
				}

				select {
				case <-ctx.Done():
					return
				case ch <- entry:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		l.log.WithFields(logrus.Fields{
			"cluster":   filter.Environment,
			"namespace": namespace,
			"container": container,
		}).Infof("closing subscription with explicit message")
		ch <- &WorkloadLogLine{
			Time:     time.Now(),
			Message:  "Subscription closed.",
			Instance: "api",
		}
		close(ch)
	}()
	return ch, nil
}

func getPods(ctx context.Context, client v1.CoreV1Interface, filter *WorkloadLogSubscriptionFilter) ([]corev1.Pod, error) {
	labelSelector := ""
	switch {
	case filter.Application != nil:
		labelSelector = "app=" + *filter.Application
	case filter.Job != nil:
		labelSelector = "app=" + *filter.Job
	}

	pods, err := client.Pods(filter.Team.String()).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if pod.Labels["logs.nais.io/flow-secure_logs"] == "true" {
			return nil, apierror.Errorf("Logs are secure, cannot be streamed.")
		}
	}

	return pods.Items, nil
}
