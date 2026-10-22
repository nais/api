package event

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/workload"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	eventv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestAddPodWorkload(t *testing.T) {
	for _, test := range []struct {
		name        string
		ownerKind   string
		label       string
		uid         types.UID
		wantKind    string
		environment string
		missingPod  bool
	}{
		{name: "application", ownerKind: "ReplicaSet", label: "app", uid: "pod-uid", wantKind: "Application"},
		{name: "job", ownerKind: "Job", label: "job", uid: "pod-uid", wantKind: "Naisjob"},
		{name: "unknown owner", ownerKind: "Other", label: "app", uid: "pod-uid"},
		{name: "missing label", ownerKind: "ReplicaSet", uid: "pod-uid"},
		{name: "replaced pod", ownerKind: "ReplicaSet", label: "app", uid: "old-pod-uid"},
		{name: "missing pod", ownerKind: "ReplicaSet", label: "app", uid: "pod-uid", missingPod: true},
		{name: "wrong environment", ownerKind: "ReplicaSet", label: "app", uid: "pod-uid", environment: "prod"},
	} {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1.Pod{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}, ObjectMeta: metav1.ObjectMeta{
				Name: "pod", Namespace: "team", UID: "pod-uid",
				Labels:          map[string]string{"app": test.label},
				OwnerReferences: []metav1.OwnerReference{{Kind: test.ownerKind, Name: "owner", Controller: new(true)}},
			}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "image"}}}}
			if test.missingPod {
				pod = nil
			}
			w := &Watcher{podWatcher: newTestPodWatcher(t, pod), log: logrus.New()}
			e := &eventv1.Event{Regarding: corev1.ObjectReference{Kind: "Pod", Name: "pod", Namespace: "team", UID: test.uid}}
			data := map[string]any{"container": "main"}
			environment := test.environment
			if environment == "" {
				environment = "dev"
			}
			w.addPodWorkload(environment, e, data)
			if data["container"] != "main" {
				t.Errorf("got container %v, want main", data["container"])
			}
			if test.wantKind == "" {
				if _, ok := data["workloadKind"]; ok {
					t.Error("unexpected workloadKind")
				}
				if _, ok := data["workloadName"]; ok {
					t.Error("unexpected workloadName")
				}
			} else {
				if data["workloadKind"] != test.wantKind {
					t.Errorf("got workload kind %v, want %s", data["workloadKind"], test.wantKind)
				}
				if data["workloadName"] != test.label {
					t.Errorf("got workload name %v, want %s", data["workloadName"], test.label)
				}
			}
		})
	}
}

func newTestPodWatcher(t *testing.T, pod *corev1.Pod) *watcher.Watcher[*corev1.Pod] {
	t.Helper()
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	clientCreator := fake.Clients(nil)
	if pod != nil {
		data, err := json.Marshal(pod)
		if err != nil {
			t.Fatal(err)
		}
		clientCreator = fake.Clients(fstest.MapFS{"dev/team/pod.yaml": &fstest.MapFile{Data: data}})
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	mgr, err := watcher.NewManager(scheme, kubernetes.ClusterConfigMap{"dev": nil}, log, watcher.WithClientCreator(clientCreator))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	podWatcher := workload.NewWatcher(ctx, mgr)
	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
	defer waitCancel()
	if !mgr.WaitForReady(waitCtx) {
		t.Fatal("pod watcher cache did not sync")
	}
	return podWatcher
}
