package leaderelection

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var elector *leaderelection.LeaderElector

func Start(ctx context.Context, client kubernetes.Interface, leaseName, namespace string, log logrus.FieldLogger) error {
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	elector, err = leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(context.Context) {
				log.Info("Started leading")
			},
			OnStoppedLeading: func() {
				log.Info("Stopped leading")
			},
			OnNewLeader: func(identity string) {
				log.Infof("New leader: %s", identity)
			},
		},
		Name: "leader-election",
	})
	if err != nil {
		return err
	}

	go elector.Run(ctx)

	return nil
}

func IsLeader() bool {
	if elector == nil {
		return false
	}

	return elector.IsLeader()
}