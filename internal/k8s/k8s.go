package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	sql_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/sql/v1beta1"
	storage_cnrm_cloud_google_com_v1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/clients/generated/apis/storage/v1beta1"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	google_nais_io_v1 "github.com/nais/liberator/pkg/apis/google.nais.io/v1"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	batchv1inf "k8s.io/client-go/informers/batch/v1"
	corev1inf "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Database interface {
	GetUserTeams(ctx context.Context, userID uuid.UUID) ([]*database.UserTeam, error)
	TeamExists(ctx context.Context, team slug.Slug) (bool, error)
}

type ClusterInformers map[string]*Informers

type ExternalResourceInformers []struct {
	GroupVersion schema.GroupVersion
	Resource     string
}

func ExternalInformers() ExternalResourceInformers {
	return ExternalResourceInformers{
		{
			GroupVersion: kafka_nais_io_v1.GroupVersion,
			Resource:     "topics",
		},
		{
			GroupVersion: aiven_io_v1alpha1.GroupVersion,
			Resource:     "redis",
		},
		{
			GroupVersion: aiven_io_v1alpha1.GroupVersion,
			Resource:     "opensearches",
		},
	}
}

func (c ClusterInformers) Start(ctx context.Context, log logrus.FieldLogger) error {
	for cluster, informer := range c {
		log := log.WithField("cluster", cluster)
		log.Infof("starting informers")
		go informer.Pod.Informer().Run(ctx.Done())
		go informer.App.Informer().Run(ctx.Done())
		go informer.Naisjob.Informer().Run(ctx.Done())
		go informer.Job.Informer().Run(ctx.Done())

		if informer.Bucket != nil {
			log.WithField("informer", "bucket").Debugf("started informer")
			go informer.Bucket.Informer().Run(ctx.Done())
		}

		if informer.BigQuery != nil {
			log.WithField("informer", "bigquerydataset").Debugf("started informer")
			go informer.BigQuery.Informer().Run(ctx.Done())
		}

		if informer.SqlInstance != nil {
			log.WithField("informer", "sqlinstance").Debugf("started informer")
			go informer.SqlInstance.Informer().Run(ctx.Done())
		}

		if informer.SqlDatabase != nil {
			log.WithField("informer", "sqldatabase").Debugf("started informer")
			go informer.SqlDatabase.Informer().Run(ctx.Done())
		}

		if informer.KafkaTopic != nil {
			log.WithField("informer", "kafkatopic").Debugf("started informer")
			go informer.KafkaTopic.Informer().Run(ctx.Done())
		}

		if informer.Redis != nil {
			log.WithField("informer", "redis").Debugf("started informer")
			go informer.Redis.Informer().Run(ctx.Done())
		}

		if informer.OpenSearch != nil {
			log.WithField("informer", "opensearch").Debugf("started informer")
			go informer.OpenSearch.Informer().Run(ctx.Done())
		}
	}

	for env, informers := range c {
		for !informers.App.Informer().HasSynced() {
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
	informers                  ClusterInformers
	clientSets                 map[string]clients
	log                        logrus.FieldLogger
	database                   Database
	impersonationClientCreator impersonationClientCreator
}

type Informers struct {
	App         informers.GenericInformer
	Event       corev1inf.EventInformer
	Job         batchv1inf.JobInformer
	Naisjob     informers.GenericInformer
	Pod         corev1inf.PodInformer
	Bucket      informers.GenericInformer
	BigQuery    informers.GenericInformer
	KafkaTopic  informers.GenericInformer
	SqlInstance informers.GenericInformer
	SqlDatabase informers.GenericInformer
	OpenSearch  informers.GenericInformer
	Redis       informers.GenericInformer
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

type clients struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
}

type impersonationClientCreator = func(context.Context) (map[string]clients, error)

func New(tenant string, cfg Config, db Database, fake bool, log logrus.FieldLogger, opts ...Opt) (*Client, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}
	// impersonationClientCreator is only nil when not using fake
	var impersonationClientCreator impersonationClientCreator = nil
	// s.clientsCreator is only nil when not using fake
	if s.clientsCreator == nil {
		restConfigs, err := CreateClusterConfigMap(tenant, cfg)
		if err != nil {
			return nil, fmt.Errorf("create kubeconfig: %w", err)
		}

		impersonationClientCreator = func(ctx context.Context) (map[string]clients, error) {
			actor := authz.ActorFromContext(ctx)
			teams, err := db.GetUserTeams(ctx, actor.User.GetID())
			if err != nil {
				return nil, err
			}

			groups := make([]string, 0)
			for _, team := range teams {
				if team.GoogleGroupEmail != nil {
					groups = append(groups, *team.GoogleGroupEmail)
				}
			}

			clientSets := make(map[string]clients)
			for cluster, restConfig := range restConfigs {
				restConfig.Impersonate = rest.ImpersonationConfig{
					UserName: actor.User.Identity(),
					Groups:   groups,
				}

				clientSet, err := kubernetes.NewForConfig(&restConfig)
				if err != nil {
					return nil, fmt.Errorf("create impersonated client: %w", err)
				}

				dynamicClient, err := dynamic.NewForConfig(&restConfig)
				if err != nil {
					return nil, fmt.Errorf("create impersonated dynamic client: %w", err)
				}

				clientSets[cluster] = clients{
					client:        clientSet,
					dynamicClient: dynamicClient,
				}
			}

			return clientSets, nil
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
	clientSets := map[string]clients{}
	for _, cluster := range clusters(cfg) {
		infs[cluster] = &Informers{}

		clientSet, dynamicClient, err := s.clientsCreator(cluster)
		if err != nil {
			return nil, fmt.Errorf("create clientsets: %w", err)
		}

		log.WithField("cluster", cluster).Debug("creating informers")
		dinf := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 4*time.Hour)
		inf := informers.NewSharedInformerFactoryWithOptions(clientSet, 4*time.Hour)

		infs[cluster].Pod = inf.Core().V1().Pods()
		infs[cluster].App = dinf.ForResource(nais_io_v1alpha1.GroupVersion.WithResource("applications"))
		infs[cluster].Naisjob = dinf.ForResource(nais_io_v1.GroupVersion.WithResource("naisjobs"))
		infs[cluster].Job = inf.Batch().V1().Jobs()

		if cfg.IsGcp(cluster) {
			infs[cluster].SqlInstance = dinf.ForResource(sql_cnrm_cloud_google_com_v1beta1.SchemeGroupVersion.WithResource("sqlinstances"))
			infs[cluster].SqlDatabase = dinf.ForResource(sql_cnrm_cloud_google_com_v1beta1.SchemeGroupVersion.WithResource("sqldatabases"))
			infs[cluster].Bucket = dinf.ForResource(storage_cnrm_cloud_google_com_v1beta1.SchemeGroupVersion.WithResource("storagebuckets"))
			infs[cluster].BigQuery = dinf.ForResource(google_nais_io_v1.GroupVersion.WithResource("bigquerydatasets"))
		}

		clientSets[cluster] = clients{
			client:        clientSet,
			dynamicClient: dynamicClient,
		}

		if clientSet, ok := clientSet.(*kubernetes.Clientset); ok {
			if err := externalInformerResources(clientSet, infs[cluster], dinf); err != nil {
				return nil, err
			}
		} else if fake {
			for _, externalResourceInformer := range ExternalInformers() {
				switch externalResourceInformer.Resource {
				case "topics":
					infs[cluster].KafkaTopic = dinf.ForResource(externalResourceInformer.GroupVersion.WithResource(externalResourceInformer.Resource))
				case "redis":
					infs[cluster].Redis = dinf.ForResource(externalResourceInformer.GroupVersion.WithResource(externalResourceInformer.Resource))
				case "opensearches":
					infs[cluster].OpenSearch = dinf.ForResource(externalResourceInformer.GroupVersion.WithResource(externalResourceInformer.Resource))
				}
			}

			log.WithField("cluster", cluster).Warn("impersonation disabled; using fake clients")
			impersonationClientCreator = func(ctx context.Context) (map[string]clients, error) {
				return clientSets, nil
			}
		}
	}

	return &Client{
		informers:                  infs,
		clientSets:                 clientSets,
		log:                        log,
		database:                   db,
		impersonationClientCreator: impersonationClientCreator,
	}, nil
}

func externalInformerResources(clientSet *kubernetes.Clientset, inf *Informers, dinf dynamicinformer.DynamicSharedInformerFactory,
) error {
	for _, externalResourceInformer := range ExternalInformers() {
		resources, err := discovery.NewDiscoveryClient(clientSet.RESTClient()).ServerResourcesForGroupVersion(externalResourceInformer.GroupVersion.String())
		if err != nil && !strings.Contains(err.Error(), "the server could not find the requested resource") {
			return fmt.Errorf("get server resources for group version: %w", err)
		}
		if err == nil {
			for _, r := range resources.APIResources {
				if r.Name != externalResourceInformer.Resource {
					continue
				}
				switch externalResourceInformer.Resource {
				case "topics":
					inf.KafkaTopic = dinf.ForResource(externalResourceInformer.GroupVersion.WithResource(externalResourceInformer.Resource))
				case "redis":
					inf.Redis = dinf.ForResource(externalResourceInformer.GroupVersion.WithResource(externalResourceInformer.Resource))
				case "opensearches":
					inf.OpenSearch = dinf.ForResource(externalResourceInformer.GroupVersion.WithResource(externalResourceInformer.Resource))
				}
			}
		}
	}
	return nil
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

func (c *Client) error(_ context.Context, err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}
