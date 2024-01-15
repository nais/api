package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	naisv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	naisv1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	batchv1inf "k8s.io/client-go/informers/batch/v1"
	corev1inf "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

type TeamChecker interface {
	TeamExists(ctx context.Context, team slug.Slug) (bool, error)
}

type ClusterInformers map[string]*Informers

func (c ClusterInformers) Start(ctx context.Context, log logrus.FieldLogger) error {
	for cluster, informer := range c {
		log.WithField("cluster", cluster).Infof("starting informers")
		go informer.PodInformer.Informer().Run(ctx.Done())
		go informer.AppInformer.Informer().Run(ctx.Done())
		go informer.NaisjobInformer.Informer().Run(ctx.Done())
		go informer.JobInformer.Informer().Run(ctx.Done())
		if informer.TopicInformer != nil {
			go informer.TopicInformer.Informer().Run(ctx.Done())
		}
	}

	for env, informers := range c {
		for !informers.AppInformer.Informer().HasSynced() {
			log.Infof("waiting for app informer in %q to sync", env)

			select {
			case <-ctx.Done():
				return fmt.Errorf("informers not started: %w", ctx.Err())
			default:
				time.Sleep(2 * time.Second)
			}
		}
	}

	return nil
}

type Client struct {
	informers   ClusterInformers
	clientSets  map[string]kubernetes.Interface
	log         logrus.FieldLogger
	teamChecker TeamChecker
}

type Informers struct {
	AppInformer     informers.GenericInformer
	EventInformer   corev1inf.EventInformer
	JobInformer     batchv1inf.JobInformer
	NaisjobInformer informers.GenericInformer
	PodInformer     corev1inf.PodInformer
	SecretInformer  corev1inf.SecretInformer
	TopicInformer   informers.GenericInformer
}

type settings struct {
	clientsCreator func(cluster string) (kubernetes.Interface, dynamic.Interface, error)
}

type Opt func(*settings)

func WithClientsCreator(f func(cluster string) (kubernetes.Interface, dynamic.Interface, error)) Opt {
	return func(s *settings) {
		s.clientsCreator = f
	}
}

func New(tenant string, cfg Config, teamChecker TeamChecker, log logrus.FieldLogger, opts ...Opt) (*Client, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}

	if s.clientsCreator == nil {
		restConfigs, err := CreateClusterConfigMap(tenant, cfg)
		if err != nil {
			return nil, fmt.Errorf("create kubeconfig: %w", err)
		}

		s.clientsCreator = func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
			restConfig := restConfigs[cluster]
			clientSet, err := kubernetes.NewForConfig(&restConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("create clientset: %w", err)
			}

			dynamicClient, err := dynamic.NewForConfig(&restConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("create dynamic client: %w", err)
			}

			return clientSet, dynamicClient, nil
		}
	}

	infs := map[string]*Informers{}
	clientSets := map[string]kubernetes.Interface{}
	for _, cluster := range clusters(cfg) {
		infs[cluster] = &Informers{}

		clientSet, dynamicClient, err := s.clientsCreator(cluster)
		if err != nil {
			return nil, fmt.Errorf("create clientsets: %w", err)
		}

		log.WithField("cluster", cluster).Debug("creating informers")
		dinf := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 4*time.Hour)
		inf := informers.NewSharedInformerFactoryWithOptions(clientSet, 4*time.Hour)

		infs[cluster].PodInformer = inf.Core().V1().Pods()
		infs[cluster].AppInformer = dinf.ForResource(naisv1alpha1.GroupVersion.WithResource("applications"))
		infs[cluster].NaisjobInformer = dinf.ForResource(naisv1.GroupVersion.WithResource("naisjobs"))
		infs[cluster].JobInformer = inf.Batch().V1().Jobs()
		infs[cluster].SecretInformer = inf.Core().V1().Secrets()
		clientSets[cluster] = clientSet

		if clientSet, ok := clientSet.(*kubernetes.Clientset); ok {
			resources, err := discovery.NewDiscoveryClient(clientSet.RESTClient()).ServerResourcesForGroupVersion(kafka_nais_io_v1.GroupVersion.String())
			if err != nil && !strings.Contains(err.Error(), "the server could not find the requested resource") {
				return nil, fmt.Errorf("get server resources for group version: %w", err)
			}
			if err == nil {
				for _, r := range resources.APIResources {
					if r.Name == "topics" {
						infs[cluster].TopicInformer = dinf.ForResource(kafka_nais_io_v1.GroupVersion.WithResource("topics"))
					}
				}
			}
		}
	}

	return &Client{
		informers:   infs,
		log:         log,
		clientSets:  clientSets,
		teamChecker: teamChecker,
	}, nil
}

func (c *Client) Search(ctx context.Context, q string, filter *model.SearchFilter) []*search.Result {
	if !isFilterOrNoFilter(filter) {
		return nil
	}

	if c.teamChecker == nil {
		panic("team checker not set")
	}

	ret := []*search.Result{}

	for env, infs := range c.informers {
		if isFilterNaisjobOrNoFilter(filter) {
			jobs, err := infs.NaisjobInformer.Lister().List(labels.Everything())
			if err != nil {
				c.error(ctx, err, "listing jobs")
				return nil
			}

			for _, obj := range jobs {
				u := obj.(*unstructured.Unstructured)
				rank := search.Match(q, u.GetName())
				if rank == -1 {
					continue
				}
				job, err := c.ToNaisJob(u, env)
				if err != nil {
					c.error(ctx, err, "converting to job")
					return nil
				} else if ok, _ := c.teamChecker.TeamExists(ctx, job.GQLVars.Team); !ok {
					continue
				}

				ret = append(ret, &search.Result{
					Node: job,
					Rank: rank,
				})
			}
		}

		if isFilterAppOrNoFilter(filter) {
			apps, err := infs.AppInformer.Lister().List(labels.Everything())
			if err != nil {
				c.error(ctx, err, "listing applications")
				return nil
			}

			for _, obj := range apps {
				u := obj.(*unstructured.Unstructured)
				rank := search.Match(q, u.GetName())
				if rank == -1 {
					continue
				}
				app, err := c.toApp(ctx, u, env)
				if err != nil {
					c.error(ctx, err, "converting to app")
					return nil
				} else if ok, _ := c.teamChecker.TeamExists(ctx, app.GQLVars.Team); !ok {
					continue
				}

				ret = append(ret, &search.Result{
					Node: app,
					Rank: rank,
				})
			}
		}

	}
	return ret
}

// Informers returns a map of informers, keyed by environment
func (c *Client) Informers() ClusterInformers {
	return c.informers
}

// convert takes a map[string]any / json, and converts it to the target struct
func convert(m any, target any) error {
	j, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshalling struct: %w", err)
	}
	err = json.Unmarshal(j, &target)
	if err != nil {
		return fmt.Errorf("unmarshalling json: %w", err)
	}
	return nil
}

func (c *Client) error(ctx context.Context, err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}

func isFilter(filter *model.SearchFilter) bool {
	if filter == nil {
		return false
	}

	if filter.Type == nil {
		return false
	}

	return true
}

func isFilterOrNoFilter(filter *model.SearchFilter) bool {
	if !isFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeApp || *filter.Type == model.SearchTypeNaisjob
}

func isFilterAppOrNoFilter(filter *model.SearchFilter) bool {
	if !isFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeApp
}

func isFilterNaisjobOrNoFilter(filter *model.SearchFilter) bool {
	if !isFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeNaisjob
}
