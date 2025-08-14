package watcher

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
)

type clusterManager struct {
	config                   *rest.Config
	client                   dynamic.Interface
	resourceMapper           KindResolver
	createdInformer          dynamicinformer.DynamicSharedInformerFactory
	createdFilteredInformers []dynamicinformer.DynamicSharedInformerFactory
	scheme                   *runtime.Scheme
	log                      logrus.FieldLogger
}

func newClusterManager(scheme *runtime.Scheme, client dynamic.Interface, discoveryClient KindResolver, config *rest.Config, log logrus.FieldLogger) (*clusterManager, error) {
	return &clusterManager{
		config:         config,
		client:         client,
		scheme:         scheme,
		log:            log,
		resourceMapper: discoveryClient,
	}, nil
}

func (c *clusterManager) informer() dynamicinformer.DynamicSharedInformerFactory {
	if c.createdInformer == nil {
		c.createdInformer = dynamicinformer.NewDynamicSharedInformerFactory(c.client, 4*time.Hour)
	}
	return c.createdInformer
}

func (c *clusterManager) filteredInformer(lblSelector string) dynamicinformer.DynamicSharedInformerFactory {
	inf := dynamicinformer.NewFilteredDynamicSharedInformerFactory(c.client, 4*time.Hour, v1.NamespaceAll, func(lo *v1.ListOptions) {
		lo.LabelSelector = lblSelector
	})
	c.createdFilteredInformers = append(c.createdFilteredInformers, inf)
	return inf
}

func (c *clusterManager) gvk(obj runtime.Object) schema.GroupVersionKind {
	gvks, _, err := c.scheme.ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		c.log.WithError(err).Info("failed to get GVKs")
		return schema.GroupVersionKind{}
	}

	return gvks[0]
}

func (c *clusterManager) createInformer(obj runtime.Object, gvr *schema.GroupVersionResource, lblSelector string) (informers.GenericInformer, schema.GroupVersionResource, error) {
	if gvr == nil {
		gvk := c.gvk(obj)
		if gvk.Empty() {
			return nil, schema.GroupVersionResource{}, fmt.Errorf("failed to get GVK for object")
		}

		gvrs, _ := meta.UnsafeGuessKindToResource(gvk)
		gvr = &gvrs
	}

	if c.resourceMapper != nil {
		// Check if the resource is available in the cluster. Will only be used when client is not a fake client
		_, err := c.resourceMapper.KindsFor(*gvr)
		if err != nil {
			c.log.WithError(err).WithField("resource", gvr.String()).Error("resource not available in cluster")
			return nil, *gvr, fmt.Errorf("resource not available in cluster")
		}
	}

	if lblSelector == "" {
		c.log.WithField("resource", gvr.String()).Debug("creating informer")
		return c.informer().ForResource(*gvr), *gvr, nil
	}

	c.log.WithField("resource", gvr.String()).Debug("creating filtered informer")
	return c.filteredInformer(lblSelector).ForResource(*gvr), *gvr, nil
}
