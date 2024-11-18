package watcher

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
)

type clusterManager struct {
	config    *rest.Config
	client    dynamic.Interface
	discovery Discovery
	informer  dynamicinformer.DynamicSharedInformerFactory
	scheme    *runtime.Scheme
	log       logrus.FieldLogger
}

func newClusterManager(scheme *runtime.Scheme, client dynamic.Interface, discoveryClient Discovery, config *rest.Config, log logrus.FieldLogger) (*clusterManager, error) {
	informer := dynamicinformer.NewDynamicSharedInformerFactory(client, 4*time.Hour)

	return &clusterManager{
		config:    config,
		client:    client,
		informer:  informer,
		log:       log,
		discovery: discoveryClient,
	}, nil
}

func (c *clusterManager) createInformer(obj runtime.Object, gvr schema.GroupVersionResource) (informers.GenericInformer, error) {
	if c.discovery != nil {
		// Check if the resource is available in the cluster. Will only be used when client is not a fake client
		_, err := c.discovery.ServerResourcesForGroupVersion(gvr.GroupVersion().String())
		if err != nil {
			c.log.WithError(err).WithField("resource", gvr.String()).Error("resource not available in cluster")
			return nil, fmt.Errorf("resource not available in cluster")
		}
	}

	c.log.WithField("resource", gvr.String()).Info("creating informer")
	return c.informer.ForResource(gvr), nil
}
