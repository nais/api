package watcher

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	schemepkg "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type clusterManager struct {
	client   dynamic.Interface
	informer dynamicinformer.DynamicSharedInformerFactory
	scheme   *runtime.Scheme
	log      logrus.FieldLogger
}

func newClusterManager(client dynamic.Interface, scheme *runtime.Scheme, config *rest.Config, log logrus.FieldLogger) (*clusterManager, error) {
	if client == nil {
		var err error
		client, err = dynamic.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("creating dynamic client: %w", err)
		}

		if config.GroupVersion == nil {
			config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
		}
		if config.NegotiatedSerializer == nil {
			config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: schemepkg.Codecs}
		}
	}

	informer := dynamicinformer.NewDynamicSharedInformerFactory(client, 4*time.Hour)

	return &clusterManager{
		client:   client,
		informer: informer,
		scheme:   scheme,
		log:      log,
	}, nil
}

func (c *clusterManager) gvk(obj runtime.Object) schema.GroupVersionKind {
	gvks, _, err := c.scheme.ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		c.log.WithError(err).Info("failed to get GVKs")
		return schema.GroupVersionKind{}
	}

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
		c.log.WithField("resource", gvr.String()).Info("creating informer")
		return c.informer.ForResource(*gvr), nil
	}
	gvk := c.gvk(obj)
	if gvk.Empty() {
		return nil, fmt.Errorf("failed to get GVK for object")
	}

	plural, _ := meta.UnsafeGuessKindToResource(gvk)
	c.log.WithField("resource", plural.String()).Info("creating informer")
	return c.informer.ForResource(plural), nil
}
