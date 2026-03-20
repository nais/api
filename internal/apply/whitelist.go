package apply

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AllowedResource identifies a Kubernetes resource by its apiVersion and kind.
type AllowedResource struct {
	APIVersion string
	Kind       string
}

// allowedResources is the single source of truth for which resources can be applied
// through the API. Each entry maps an apiVersion+kind pair to its GroupVersionResource,
// avoiding the need for a discovery client.
var allowedResources = map[AllowedResource]schema.GroupVersionResource{
	// Core workloads
	{APIVersion: "nais.io/v1alpha1", Kind: "Application"}: {
		Group: "nais.io", Version: "v1alpha1", Resource: "applications",
	},
	{APIVersion: "nais.io/v1", Kind: "Naisjob"}: {
		Group: "nais.io", Version: "v1", Resource: "naisjobs",
	},

	// Aiven
	{APIVersion: "aiven.io/v1alpha1", Kind: "OpenSearch"}: {
		Group: "aiven.io", Version: "v1alpha1", Resource: "opensearches",
	},
	{APIVersion: "aiven.io/v1alpha1", Kind: "ServiceIntegration"}: {
		Group: "aiven.io", Version: "v1alpha1", Resource: "serviceintegrations",
	},
	{APIVersion: "aiven.io/v1alpha1", Kind: "Valkey"}: {
		Group: "aiven.io", Version: "v1alpha1", Resource: "valkeys",
	},
	{APIVersion: "aiven.nais.io/v1", Kind: "AivenApplication"}: {
		Group: "aiven.nais.io", Version: "v1", Resource: "aivenapplications",
	},

	// Autoscaling
	{APIVersion: "autoscaling/v2", Kind: "HorizontalPodAutoscaler"}: {
		Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers",
	},

	// Batch
	{APIVersion: "batch/v1", Kind: "Job"}: {
		Group: "batch", Version: "v1", Resource: "jobs",
	},

	// BigQuery (Config Connector)
	{APIVersion: "bigquery.cnrm.cloud.google.com/v1beta1", Kind: "BigQueryTable"}: {
		Group: "bigquery.cnrm.cloud.google.com", Version: "v1beta1", Resource: "bigquerytables",
	},

	// Postgres (NAIS)
	{APIVersion: "data.nais.io/v1", Kind: "Postgres"}: {
		Group: "data.nais.io", Version: "v1", Resource: "postgres",
	},

	// IAM (Config Connector)
	{APIVersion: "iam.cnrm.cloud.google.com/v1beta1", Kind: "IAMPolicyMember"}: {
		Group: "iam.cnrm.cloud.google.com", Version: "v1beta1", Resource: "iampolicymembers",
	},

	// Kafka
	{APIVersion: "kafka.nais.io/v1", Kind: "Topic"}: {
		Group: "kafka.nais.io", Version: "v1", Resource: "topics",
	},

	// Krakend
	{APIVersion: "krakend.nais.io/v1", Kind: "ApiEndpoints"}: {
		Group: "krakend.nais.io", Version: "v1", Resource: "apiendpoints",
	},
	{APIVersion: "krakend.nais.io/v1", Kind: "Krakend"}: {
		Group: "krakend.nais.io", Version: "v1", Resource: "krakends",
	},

	// Logging (Config Connector)
	{APIVersion: "logging.cnrm.cloud.google.com/v1beta1", Kind: "LoggingLogSink"}: {
		Group: "logging.cnrm.cloud.google.com", Version: "v1beta1", Resource: "logginglogsinks",
	},

	// Monitoring (Prometheus Operator)
	{APIVersion: "monitoring.coreos.com/v1", Kind: "Probe"}: {
		Group: "monitoring.coreos.com", Version: "v1", Resource: "probes",
	},
	{APIVersion: "monitoring.coreos.com/v1", Kind: "PrometheusRule"}: {
		Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules",
	},
	{APIVersion: "monitoring.coreos.com/v1", Kind: "ServiceMonitor"}: {
		Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors",
	},
	{APIVersion: "monitoring.coreos.com/v1alpha1", Kind: "AlertmanagerConfig"}: {
		Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "alertmanagerconfigs",
	},

	// NAIS
	{APIVersion: "nais.io/v1", Kind: "AzureAdApplication"}: {
		Group: "nais.io", Version: "v1", Resource: "azureadapplications",
	},
	{APIVersion: "nais.io/v1", Kind: "Image"}: {
		Group: "nais.io", Version: "v1", Resource: "images",
	},
	{APIVersion: "nais.io/v1alpha1", Kind: "Alerts"}: {
		Group: "nais.io", Version: "v1alpha1", Resource: "alerts",
	},
	{APIVersion: "nais.io/v1alpha1", Kind: "Naisjob"}: {
		Group: "nais.io", Version: "v1alpha1", Resource: "naisjobs",
	},

	// Networking
	{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"}: {
		Group: "networking.k8s.io", Version: "v1", Resource: "ingresses",
	},
	{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"}: {
		Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies",
	},

	// PubSub (Config Connector)
	{APIVersion: "pubsub.cnrm.cloud.google.com/v1beta1", Kind: "PubSubTopic"}: {
		Group: "pubsub.cnrm.cloud.google.com", Version: "v1beta1", Resource: "pubsubtopics",
	},

	// RBAC
	{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"}: {
		Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles",
	},
	{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"}: {
		Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings",
	},
	{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "Role"}: {
		Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles",
	},
	{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding"}: {
		Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings",
	},

	// Unleash
	{APIVersion: "unleash.nais.io/v1", Kind: "ApiToken"}: {
		Group: "unleash.nais.io", Version: "v1", Resource: "apitokens",
	},

	// Core v1
	{APIVersion: "v1", Kind: "ConfigMap"}: {
		Group: "", Version: "v1", Resource: "configmaps",
	},
	{APIVersion: "v1", Kind: "Endpoints"}: {
		Group: "", Version: "v1", Resource: "endpoints",
	},
	{APIVersion: "v1", Kind: "PersistentVolumeClaim"}: {
		Group: "", Version: "v1", Resource: "persistentvolumeclaims",
	},
	{APIVersion: "v1", Kind: "Secret"}: {
		Group: "", Version: "v1", Resource: "secrets",
	},
	{APIVersion: "v1", Kind: "Service"}: {
		Group: "", Version: "v1", Resource: "services",
	},
	{APIVersion: "v1", Kind: "ServiceAccount"}: {
		Group: "", Version: "v1", Resource: "serviceaccounts",
	},
}

// IsAllowed returns true if the given apiVersion and kind are in the whitelist.
func IsAllowed(res unstructured.Unstructured) bool {
	_, ok := allowedResources[AllowedResource{APIVersion: res.GetAPIVersion(), Kind: res.GetKind()}]
	return ok
}

// GVRFor returns the GroupVersionResource for the given apiVersion and kind.
// The second return value is false if the resource is not in the whitelist.
func GVRFor(apiVersion, kind string) (schema.GroupVersionResource, bool) {
	gvr, ok := allowedResources[AllowedResource{APIVersion: apiVersion, Kind: kind}]
	return gvr, ok
}
