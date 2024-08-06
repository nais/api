package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/nais/api/internal/v1/kubernetes"
	"github.com/nais/api/internal/v1/kubernetes/fake"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	scheme, err := kubernetes.NewScheme()
	if err != nil {
		panic(err)
	}

	// log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	log := logrus.New()
	log.Level = logrus.DebugLevel

	mgr, err := watcher.NewManager(scheme, "local", watcher.Config{
		StaticClusters: []watcher.StaticCluster{
			{
				Name: "dev",
			},
		},
	}, log, watcher.WithClientCreator(fake.Clients(os.DirFS("./data/k8s"))))
	if err != nil {
		panic(err)
	}

	// appWatcher := watcher.Watch(mgr, &CustomApp{}, watcher.WithConverter(toCustomApp), watcher.WithGVR(schema.GroupVersionResource{
	// 	Group:    "nais.io",
	// 	Version:  "v1alpha1",
	// 	Resource: "applications",
	// }))

	appWatcher := watcher.Watch(mgr, &nais_io_v1alpha1.Application{})

	fmt.Println("Starting appWatcher")
	appWatcher.Start(ctx)

	fmt.Println("Waiting for pod cache to sync")
	appWatcher.WaitForReady(ctx, 10*time.Second)

	go func() {
		time.Sleep(3 * time.Second)
		fmt.Println("Test getting")

		spew.Dump(appWatcher.GetByNamespace("devteam"))
	}()

	fmt.Println("Listening for pod changes")
	<-ctx.Done()
	mgr.Stop()
}

type CustomApp struct {
	Name      string
	Namespace string
	Labels    map[string]string

	Image string
}

func (c *CustomApp) GetName() string { return c.Name }

func (c *CustomApp) GetNamespace() string { return c.Namespace }

func (c *CustomApp) GetLabels() map[string]string { return c.Labels }

func (c *CustomApp) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (c *CustomApp) DeepCopyObject() runtime.Object {
	return &CustomApp{
		Name:      c.Name,
		Namespace: c.Namespace,
		Labels:    c.Labels,
		Image:     c.Image,
	}
}

func toCustomApp(u *unstructured.Unstructured) (any, bool) {
	if u.GetKind() != "Application" {
		return nil, false
	}

	app := &CustomApp{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
		Labels:    u.GetLabels(),
	}

	image, ok, err := unstructured.NestedString(u.Object, "spec", "image")
	if !ok || err != nil {
		fmt.Println("failed to get containers", err)
		return nil, false
	}
	app.Image = image

	return app, true
}
