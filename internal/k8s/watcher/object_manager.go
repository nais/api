package watcher

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/davecgh/go-spew/spew"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	schemepkg "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type clusterManager struct {
	client     dynamic.Interface
	restClient rest.Interface
	informer   dynamicinformer.DynamicSharedInformerFactory
	log        *slog.Logger

	serverGroups *metav1.APIGroupList
	scheme       *runtime.Scheme
}

func newClusterManager(scheme *runtime.Scheme, config *rest.Config, log *slog.Logger) (*clusterManager, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	if config.GroupVersion == nil {
		config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	}
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: schemepkg.Codecs}
	}
	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("creating REST client: %w", err)
	}

	serverGroups, err := discovery.NewDiscoveryClient(restClient).ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("getting server groups: %w", err)
	}

	informer := dynamicinformer.NewDynamicSharedInformerFactory(client, 4*time.Hour)

	return &clusterManager{
		client:       client,
		restClient:   restClient,
		informer:     informer,
		serverGroups: serverGroups,
		scheme:       scheme,
		log:          log,
	}, nil
}

func (c *clusterManager) gvk(obj runtime.Object) schema.GroupVersionKind {
	gvks, _, err := c.scheme.ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		slog.Info("failed to get GVKs", "error", err)
		return schema.GroupVersionKind{}
	}

	spew.Dump(gvks)

	return gvks[0]
	// for _, group := range c.serverGroups.Groups {
	// 	spew.Dump(group)
	// 	fmt.Println("---")
	// 	spew.Dump(gvk)
	// 	if group.Name == gvk.Group {
	// 		if gvk.Kind == group.Kind {
	// 			return gvk
	// 		}
	// 	}
	// }

	// return schema.GroupVersionKind{}
}

func (c *clusterManager) createInformer(obj runtime.Object, gvr *schema.GroupVersionResource) (informers.GenericInformer, error) {
	if gvr != nil {
		c.log.Info("creating informer", "resource", gvr.String())
		return c.informer.ForResource(*gvr), nil
	}
	gvk := c.gvk(obj)
	if gvk.Empty() {
		return nil, fmt.Errorf("failed to get GVK for object")
	}

	plural, _ := meta.UnsafeGuessKindToResource(gvk)
	c.log.Info("creating informer", "resource", plural.String())
	return c.informer.ForResource(plural), nil
}
