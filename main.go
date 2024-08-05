package main

import (
	"context"
	"fmt"
	"github.com/nais/api/internal/v1/kubernetes/fake"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	// log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	log := logrus.New()
	log.Level = logrus.DebugLevel

	mgr, err := watcher.NewManager(scheme, "local", k8s.Config{
		StaticClusters: []k8s.StaticCluster{
			{
				Name: "dev",
			},
		},
	}, log, watcher.WithClientCreator(fake.Clients(os.DirFS("./data/k8s"))))
	if err != nil {
		panic(err)
	}

	podWatcher := watcher.Watch(mgr, &CustomPod{}, watcher.WithConverter(toCustomPod), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}))

	fmt.Println("Starting podWatcher")
	podWatcher.Start(ctx)

	fmt.Println("Waiting for pod cache to sync")
	podWatcher.WaitForReady(ctx, 10*time.Second)

	go func() {
		time.Sleep(3 * time.Second)
		fmt.Println("Test getting")

		spew.Dump(podWatcher.GetByCluster("dev"))
	}()

	fmt.Println("Listening for pod changes")
	<-ctx.Done()
	mgr.Stop()
}

type CustomPod struct {
	Name      string
	Namespace string
	Labels    map[string]string

	Images []string
}

func (c *CustomPod) GetName() string { return c.Name }

func (c *CustomPod) GetNamespace() string { return c.Namespace }

func (c *CustomPod) GetLabels() map[string]string { return c.Labels }

func (c *CustomPod) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (c *CustomPod) DeepCopyObject() runtime.Object {
	return &CustomPod{
		Name:      c.Name,
		Namespace: c.Namespace,
		Labels:    c.Labels,
		Images:    c.Images,
	}
}

func toCustomPod(u *unstructured.Unstructured) (any, bool) {
	if u.GetKind() != "Pod" {
		return nil, false
	}

	pod := &CustomPod{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
		Labels:    u.GetLabels(),
	}

	containers, ok, err := unstructured.NestedSlice(u.Object, "spec", "containers")
	if !ok || err != nil {
		fmt.Println("failed to get containers", err)
		return nil, false
	}

	for _, container := range containers {
		img, ok, err := unstructured.NestedString(container.(map[string]any), "image")
		if !ok || err != nil {
			fmt.Println("failed to get image", err)
			return nil, false
		}

		pod.Images = append(pod.Images, img)
	}

	return pod, true
}
