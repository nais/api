package aivencredentials

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var aivenApplicationGVR = schema.GroupVersionResource{
	Group:    "aiven.nais.io",
	Version:  "v1",
	Resource: "aivenapplications",
}

var secretGVR = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "secrets",
}

const (
	pollInterval = 2 * time.Second
	pollTimeout  = 60 * time.Second

	maxTTLDefault = 30 * 24 * time.Hour  // 30 days — used by OpenSearch and Valkey
	maxTTLKafka   = 365 * 24 * time.Hour // 365 days — used by Kafka
)

// credentialRequest captures all the parameters needed to create credentials for any Aiven service.
type credentialRequest struct {
	teamSlug        slug.Slug
	environmentName string
	ttl             string
	service         string // "opensearch", "valkey", "kafka"
	instanceName    string // empty for kafka
	permission      string // empty for kafka
	maxTTL          time.Duration

	// buildSpec returns the AivenApplication spec for this service type.
	buildSpec func(namespace, secretName string, expiresAt time.Time) map[string]any

	// extractCreds extracts typed credentials from the secret data.
	extractCreds func(data map[string]string) any
}

// createCredentials is the shared implementation for all three credential types.
func createCredentials(ctx context.Context, req credentialRequest) (any, error) {
	ttl, err := parseTTL(req.ttl, req.maxTTL)
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx).User
	namespace := req.teamSlug.String()
	secretName := generateSecretName(actor.Identity(), namespace, req.service)
	appName := generateAppName(actor.Identity(), req.service)

	expiresAt := time.Now().Add(ttl).UTC()
	spec := req.buildSpec(namespace, secretName, expiresAt)

	client, err := getClient(ctx, req.environmentName)
	if err != nil {
		return nil, err
	}

	if err := createOrUpdateAivenApplication(ctx, client, appName, namespace, spec, actor, expiresAt); err != nil {
		return nil, fmt.Errorf("creating AivenApplication: %w", err)
	}

	secret, err := waitForSecret(ctx, client, namespace, secretName)
	if err != nil {
		return nil, fmt.Errorf("waiting for credentials: %w", err)
	}

	l := fromContext(ctx)
	data := secretData(secret, l.log)
	creds := req.extractCreds(data)

	if err := logCredentialCreation(ctx, req); err != nil {
		l.log.WithError(err).Warn("failed to create activity log entry")
	}

	return creds, nil
}

func CreateOpenSearchCredentials(ctx context.Context, input CreateOpenSearchCredentialsInput) (*CreateOpenSearchCredentialsPayload, error) {
	// Strip "opensearch-<team>-" prefix if the user provided the full Kubernetes resource name.
	// The buildSpec already prepends "opensearch-<namespace>-" for the Aivenator.
	instanceName := strings.TrimPrefix(input.InstanceName, opensearch.NamePrefix(input.TeamSlug))
	result, err := createCredentials(ctx, credentialRequest{
		teamSlug:        input.TeamSlug,
		environmentName: input.EnvironmentName,
		ttl:             input.TTL,
		service:         "opensearch",
		instanceName:    instanceName,
		permission:      input.Permission.String(),
		maxTTL:          maxTTLDefault,
		buildSpec: func(namespace, secretName string, expiresAt time.Time) map[string]any {
			return map[string]any{
				"protected": true,
				"expiresAt": expiresAt.Format(time.RFC3339),
				"openSearch": map[string]any{
					"instance":   fmt.Sprintf("opensearch-%s-%s", namespace, instanceName),
					"access":     input.Permission.aivenAccess(),
					"secretName": secretName,
				},
			}
		},
		extractCreds: func(data map[string]string) any {
			port, _ := strconv.Atoi(data["OPEN_SEARCH_PORT"])
			return &OpenSearchCredentials{
				Username: data["OPEN_SEARCH_USERNAME"],
				Password: data["OPEN_SEARCH_PASSWORD"],
				Host:     data["OPEN_SEARCH_HOST"],
				Port:     port,
				URI:      data["OPEN_SEARCH_URI"],
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return &CreateOpenSearchCredentialsPayload{Credentials: result.(*OpenSearchCredentials)}, nil
}

// valkeyEnvVarName matches the Aivenator's key naming convention for Valkey secrets.
// The Aivenator suffixes all Valkey secret keys with the uppercased instance name,
// where non-alphanumeric characters are replaced with underscores.
// For example, instance "my-cache" produces suffix "MY_CACHE", giving keys like "VALKEY_HOST_MY_CACHE".
var valkeyNamePattern = regexp.MustCompile("[^a-z0-9]")

func valkeyEnvVarSuffix(instanceName string) string {
	return strings.ToUpper(valkeyNamePattern.ReplaceAllString(instanceName, "_"))
}

func CreateValkeyCredentials(ctx context.Context, input CreateValkeyCredentialsInput) (*CreateValkeyCredentialsPayload, error) {
	// Strip "valkey-<team>-" prefix if the user provided the full Kubernetes resource name.
	// Aivenator expects the short instance name and prepends "valkey-<namespace>-" itself.
	instanceName := strings.TrimPrefix(input.InstanceName, valkey.NamePrefix(input.TeamSlug))
	suffix := valkeyEnvVarSuffix(instanceName)
	result, err := createCredentials(ctx, credentialRequest{
		teamSlug:        input.TeamSlug,
		environmentName: input.EnvironmentName,
		ttl:             input.TTL,
		service:         "valkey",
		instanceName:    instanceName,
		permission:      input.Permission.String(),
		maxTTL:          maxTTLDefault,
		buildSpec: func(namespace, secretName string, expiresAt time.Time) map[string]any {
			return map[string]any{
				"protected": true,
				"expiresAt": expiresAt.Format(time.RFC3339),
				"valkey": []any{
					map[string]any{
						"instance":   instanceName,
						"access":     input.Permission.aivenAccess(),
						"secretName": secretName,
					},
				},
			}
		},
		extractCreds: func(data map[string]string) any {
			port, _ := strconv.Atoi(data[fmt.Sprintf("VALKEY_PORT_%s", suffix)])
			return &ValkeyCredentials{
				Username: data[fmt.Sprintf("VALKEY_USERNAME_%s", suffix)],
				Password: data[fmt.Sprintf("VALKEY_PASSWORD_%s", suffix)],
				Host:     data[fmt.Sprintf("VALKEY_HOST_%s", suffix)],
				Port:     port,
				URI:      data[fmt.Sprintf("VALKEY_URI_%s", suffix)],
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return &CreateValkeyCredentialsPayload{Credentials: result.(*ValkeyCredentials)}, nil
}

func CreateKafkaCredentials(ctx context.Context, input CreateKafkaCredentialsInput) (*CreateKafkaCredentialsPayload, error) {
	result, err := createCredentials(ctx, credentialRequest{
		teamSlug:        input.TeamSlug,
		environmentName: input.EnvironmentName,
		ttl:             input.TTL,
		service:         "kafka",
		maxTTL:          maxTTLKafka,
		buildSpec: func(namespace, secretName string, expiresAt time.Time) map[string]any {
			return map[string]any{
				"protected": true,
				"expiresAt": expiresAt.Format(time.RFC3339),
				"kafka": map[string]any{
					"pool":       "nav-" + input.EnvironmentName,
					"secretName": secretName,
				},
			}
		},
		extractCreds: func(data map[string]string) any {
			return &KafkaCredentials{
				Username:       data["KAFKA_SCHEMA_REGISTRY_USER"],
				AccessCert:     data["KAFKA_CERTIFICATE"],
				AccessKey:      data["KAFKA_PRIVATE_KEY"],
				CaCert:         data["KAFKA_CA"],
				Brokers:        data["KAFKA_BROKERS"],
				SchemaRegistry: data["KAFKA_SCHEMA_REGISTRY"],
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return &CreateKafkaCredentialsPayload{Credentials: result.(*KafkaCredentials)}, nil
}

// getClient returns the dynamic client for the given environment (cluster).
func getClient(ctx context.Context, environmentName string) (dynamic.Interface, error) {
	l := fromContext(ctx)
	clusterName := environmentmapper.ClusterName(environmentName)
	client, ok := l.dynamicClients[clusterName]
	if !ok {
		return nil, fmt.Errorf("unknown environment: %s", environmentName)
	}
	return client, nil
}

// createOrUpdateAivenApplication creates or updates an AivenApplication CRD in the given namespace.
// expiresAt is used to set the euthanaisa.nais.io/kill-after label so that euthanaisa can clean up
// expired AivenApplications automatically.
func createOrUpdateAivenApplication(ctx context.Context, client dynamic.Interface, name, namespace string, spec map[string]any, actor authz.AuthenticatedUser, expiresAt time.Time) error {
	res := &unstructured.Unstructured{}
	res.SetAPIVersion("aiven.nais.io/v1")
	res.SetKind("AivenApplication")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, actor.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)
	labels := res.GetLabels()
	labels["euthanaisa.nais.io/kill-after"] = strconv.FormatInt(expiresAt.Unix(), 10)
	res.SetLabels(labels)
	res.Object["spec"] = spec

	aivenClient := client.Resource(aivenApplicationGVR).Namespace(namespace)

	existing, err := aivenClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("checking existing AivenApplication: %w", err)
		}
		// Create
		_, err = aivenClient.Create(ctx, res, metav1.CreateOptions{})
		return err
	}

	// Refuse to overwrite app-owned resources
	if len(existing.GetOwnerReferences()) > 0 {
		return fmt.Errorf("AivenApplication %s/%s is owned by another resource, refusing to overwrite", namespace, name)
	}

	// Update
	res.SetResourceVersion(existing.GetResourceVersion())
	_, err = aivenClient.Update(ctx, res, metav1.UpdateOptions{})
	return err
}

// waitForSecret polls for a Kubernetes Secret to be created and populated by the Aivenator.
// It waits until the secret exists and has a non-empty "data" field, to avoid returning
// before the Aivenator has finished writing credentials to the secret.
func waitForSecret(ctx context.Context, client dynamic.Interface, namespace, secretName string) (*unstructured.Unstructured, error) {
	secretClient := client.Resource(secretGVR).Namespace(namespace)

	deadline := time.After(pollTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, fmt.Errorf("timed out waiting for secret %s/%s to be created by Aivenator", namespace, secretName)
		case <-ticker.C:
			secret, err := secretClient.Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				return nil, fmt.Errorf("fetching secret: %w", err)
			}

			// The secret may exist before the Aivenator has populated it with data.
			// Keep polling until the data field is present and non-empty.
			data, ok := secret.Object["data"].(map[string]any)
			if !ok || len(data) == 0 {
				continue
			}

			return secret, nil
		}
	}
}

// secretData extracts the .data map from a Secret, base64-decoding the values.
// Kubernetes secrets store data as base64-encoded strings in the "data" field.
func secretData(secret *unstructured.Unstructured, log logrus.FieldLogger) map[string]string {
	result := make(map[string]string)
	data, ok := secret.Object["data"].(map[string]any)
	if !ok {
		return result
	}
	for k, v := range data {
		s, ok := v.(string)
		if !ok {
			log.WithField("key", k).WithField("type", fmt.Sprintf("%T", v)).Warn("unexpected non-string value in secret data")
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			log.WithField("key", k).WithError(err).Warn("failed to base64-decode secret value")
			result[k] = s // use raw value as fallback
			continue
		}
		result[k] = string(decoded)
	}
	return result
}

// generateSecretName creates a deterministic, short secret name.
func generateSecretName(username, namespace, service string) string {
	hash := sha256.Sum256(fmt.Appendf(nil, "%s-%s-%s", username, namespace, service))
	return fmt.Sprintf("tmp-%s-%x", service, hash[:3])
}

// generateAppName creates a deterministic AivenApplication name from the user identity.
func generateAppName(username, service string) string {
	hash := sha256.Sum256(fmt.Appendf(nil, "%s-%s", username, service))
	return fmt.Sprintf("tmp-%s-%x", service, hash[:3])
}

// parseTTL parses a human-readable TTL string (e.g. "1d", "7d", "24h") into a time.Duration.
func parseTTL(ttl string, maxTTL time.Duration) (time.Duration, error) {
	ttl = strings.TrimSpace(ttl)

	var d time.Duration
	// Support day notation
	if before, ok := strings.CutSuffix(ttl, "d"); ok {
		days, err := strconv.Atoi(before)
		if err != nil {
			return 0, apierror.Errorf("invalid TTL: %s", ttl)
		}
		d = time.Duration(days) * 24 * time.Hour
	} else {
		// Fall back to Go duration parsing (e.g. "24h", "168h")
		var err error
		d, err = time.ParseDuration(ttl)
		if err != nil {
			return 0, apierror.Errorf("invalid TTL: %s (use e.g. '1d', '7d', '24h')", ttl)
		}
	}

	if d <= 0 {
		return 0, apierror.Errorf("TTL must be positive")
	}
	if d > maxTTL {
		maxDays := int(maxTTL.Hours() / 24)
		return 0, apierror.Errorf("TTL exceeds maximum of %d days", maxDays)
	}
	return d, nil
}

// logCredentialCreation logs that credentials were created to the activity log.
func logCredentialCreation(ctx context.Context, req credentialRequest) error {
	envName := req.environmentName
	serviceType := strings.ToUpper(req.service)
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionCreateCredentials,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypeCredentials,
		ResourceName:    serviceType,
		EnvironmentName: &envName,
		TeamSlug:        &req.teamSlug,
		Data: CredentialsActivityLogEntryData{
			ServiceType:  serviceType,
			InstanceName: req.instanceName,
			Permission:   req.permission,
			TTL:          req.ttl,
		},
	})
}
