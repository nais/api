package aivencredentials

import (
	"context"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/slug"
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
	maxTTL       = 30 * 24 * time.Hour // 30 days
)

func CreateOpenSearchCredentials(ctx context.Context, input CreateOpenSearchCredentialsInput) (*CreateOpenSearchCredentialsPayload, error) {
	ttl, err := parseTTL(input.TTL)
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx).User
	namespace := input.TeamSlug.String()
	secretName := generateSecretName(actor.Identity(), namespace, "opensearch")
	appName := generateAppName(actor.Identity(), "opensearch")

	spec := map[string]any{
		"protected": true,
		"expiresAt": time.Now().Add(ttl).UTC().Format(time.RFC3339),
		"openSearch": map[string]any{
			"instance":   fmt.Sprintf("opensearch-%s-%s", namespace, input.InstanceName),
			"access":     input.Permission.aivenAccess(),
			"secretName": secretName,
		},
	}

	client, err := getClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	if err := createOrUpdateAivenApplication(ctx, client, appName, namespace, spec, actor); err != nil {
		return nil, fmt.Errorf("creating AivenApplication: %w", err)
	}

	secret, err := waitForSecret(ctx, client, namespace, secretName)
	if err != nil {
		return nil, fmt.Errorf("waiting for credentials: %w", err)
	}

	data := secretData(secret)
	port, _ := strconv.Atoi(getSecretField(data, "OPEN_SEARCH_PORT"))

	creds := &OpenSearchCredentials{
		Username: getSecretField(data, "OPEN_SEARCH_USERNAME"),
		Password: getSecretField(data, "OPEN_SEARCH_PASSWORD"),
		Host:     getSecretField(data, "OPEN_SEARCH_HOST"),
		Port:     port,
		URI:      getSecretField(data, "OPEN_SEARCH_URI"),
	}

	if err := logCredentialCreation(ctx, "OPENSEARCH", input.InstanceName, input.Permission.String(), input.TTL, input.EnvironmentName, input.TeamSlug); err != nil {
		fromContext(ctx).log.WithError(err).Warn("failed to create activity log entry")
	}

	return &CreateOpenSearchCredentialsPayload{Credentials: creds}, nil
}

func CreateValkeyCredentials(ctx context.Context, input CreateValkeyCredentialsInput) (*CreateValkeyCredentialsPayload, error) {
	ttl, err := parseTTL(input.TTL)
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx).User
	namespace := input.TeamSlug.String()
	secretName := generateSecretName(actor.Identity(), namespace, "valkey")
	appName := generateAppName(actor.Identity(), "valkey")

	spec := map[string]any{
		"protected": true,
		"expiresAt": time.Now().Add(ttl).UTC().Format(time.RFC3339),
		"valkey": []any{
			map[string]any{
				"instance":   fmt.Sprintf("valkey-%s-%s", namespace, input.InstanceName),
				"access":     input.Permission.aivenAccess(),
				"secretName": secretName,
			},
		},
	}

	client, err := getClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	if err := createOrUpdateAivenApplication(ctx, client, appName, namespace, spec, actor); err != nil {
		return nil, fmt.Errorf("creating AivenApplication: %w", err)
	}

	secret, err := waitForSecret(ctx, client, namespace, secretName)
	if err != nil {
		return nil, fmt.Errorf("waiting for credentials: %w", err)
	}

	data := secretData(secret)
	port, _ := strconv.Atoi(getSecretField(data, "VALKEY_PORT"))

	creds := &ValkeyCredentials{
		Username: getSecretField(data, "VALKEY_USERNAME"),
		Password: getSecretField(data, "VALKEY_PASSWORD"),
		Host:     getSecretField(data, "VALKEY_HOST"),
		Port:     port,
		URI:      getSecretField(data, "VALKEY_URI"),
	}

	if err := logCredentialCreation(ctx, "VALKEY", input.InstanceName, input.Permission.String(), input.TTL, input.EnvironmentName, input.TeamSlug); err != nil {
		fromContext(ctx).log.WithError(err).Warn("failed to create activity log entry")
	}

	return &CreateValkeyCredentialsPayload{Credentials: creds}, nil
}

func CreateKafkaCredentials(ctx context.Context, input CreateKafkaCredentialsInput) (*CreateKafkaCredentialsPayload, error) {
	ttl, err := parseTTL(input.TTL)
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx).User
	namespace := input.TeamSlug.String()
	secretName := generateSecretName(actor.Identity(), namespace, "kafka")
	appName := generateAppName(actor.Identity(), "kafka")

	spec := map[string]any{
		"protected": true,
		"expiresAt": time.Now().Add(ttl).UTC().Format(time.RFC3339),
		"kafka": map[string]any{
			"pool":       "nav-" + input.EnvironmentName,
			"secretName": secretName,
		},
	}

	client, err := getClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	if err := createOrUpdateAivenApplication(ctx, client, appName, namespace, spec, actor); err != nil {
		return nil, fmt.Errorf("creating AivenApplication: %w", err)
	}

	secret, err := waitForSecret(ctx, client, namespace, secretName)
	if err != nil {
		return nil, fmt.Errorf("waiting for credentials: %w", err)
	}

	data := secretData(secret)

	creds := &KafkaCredentials{
		Username:       getSecretField(data, "KAFKA_SCHEMA_REGISTRY_USER"),
		AccessCert:     getSecretField(data, "KAFKA_CERTIFICATE"),
		AccessKey:      getSecretField(data, "KAFKA_PRIVATE_KEY"),
		CaCert:         getSecretField(data, "KAFKA_CA"),
		Brokers:        getSecretField(data, "KAFKA_BROKERS"),
		SchemaRegistry: getSecretField(data, "KAFKA_SCHEMA_REGISTRY"),
	}

	if err := logCredentialCreation(ctx, "KAFKA", "", "", input.TTL, input.EnvironmentName, input.TeamSlug); err != nil {
		fromContext(ctx).log.WithError(err).Warn("failed to create activity log entry")
	}

	return &CreateKafkaCredentialsPayload{Credentials: creds}, nil
}

// getClient returns the dynamic client for the given environment (cluster).
func getClient(ctx context.Context, environmentName string) (dynamic.Interface, error) {
	l := fromContext(ctx)
	client, ok := l.dynamicClients[environmentName]
	if !ok {
		return nil, fmt.Errorf("unknown environment: %s", environmentName)
	}
	return client, nil
}

// createOrUpdateAivenApplication creates or updates an AivenApplication CRD in the given namespace.
func createOrUpdateAivenApplication(ctx context.Context, client dynamic.Interface, name, namespace string, spec map[string]any, actor authz.AuthenticatedUser) error {
	res := &unstructured.Unstructured{}
	res.SetAPIVersion("aiven.nais.io/v1")
	res.SetKind("AivenApplication")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, actor.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)
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

// waitForSecret polls for a Kubernetes Secret to be created by the Aivenator.
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
			return secret, nil
		}
	}
}

// secretData extracts the .data map from a Secret, base64-decoding the values.
func secretData(secret *unstructured.Unstructured) map[string]string {
	result := make(map[string]string)
	data, ok := secret.Object["data"].(map[string]any)
	if !ok {
		return result
	}
	for k, v := range data {
		switch val := v.(type) {
		case string:
			// unstructured secrets have already-decoded string values
			result[k] = val
		}
	}
	return result
}

// getSecretField returns the value for a key, or empty string if missing.
func getSecretField(data map[string]string, key string) string {
	return data[key]
}

// generateSecretName creates a deterministic, short secret name.
func generateSecretName(username, namespace, service string) string {
	hasher := crc32.NewIEEE()
	fmt.Fprintf(hasher, "%s-%s-%s", username, namespace, service)
	return fmt.Sprintf("aiven-%s-%08x", service, hasher.Sum32())
}

// generateAppName creates a deterministic AivenApplication name from the user identity.
func generateAppName(username, service string) string {
	name := strings.ReplaceAll(username, ".", "-")
	name = strings.ReplaceAll(name, "@", "-")
	hasher := crc32.NewIEEE()
	fmt.Fprintf(hasher, "%s-%s", username, service)
	return fmt.Sprintf("console-%s-%08x", service, hasher.Sum32())
}

// parseTTL parses a human-readable TTL string (e.g. "1d", "7d", "24h") into a time.Duration.
func parseTTL(ttl string) (time.Duration, error) {
	ttl = strings.TrimSpace(ttl)

	// Support day notation
	if strings.HasSuffix(ttl, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(ttl, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid TTL: %s", ttl)
		}
		d := time.Duration(days) * 24 * time.Hour
		if d > maxTTL {
			return 0, fmt.Errorf("TTL exceeds maximum of 30 days")
		}
		if d <= 0 {
			return 0, fmt.Errorf("TTL must be positive")
		}
		return d, nil
	}

	// Fall back to Go duration parsing (e.g. "24h", "168h")
	d, err := time.ParseDuration(ttl)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL: %s (use e.g. '1d', '7d', '24h')", ttl)
	}
	if d > maxTTL {
		return 0, fmt.Errorf("TTL exceeds maximum of 30 days")
	}
	if d <= 0 {
		return 0, fmt.Errorf("TTL must be positive")
	}
	return d, nil
}

// logCredentialCreation logs that credentials were created to the activity log.
func logCredentialCreation(ctx context.Context, serviceType, instanceName, permission, ttl, environmentName string, teamSlug slug.Slug) error {
	envName := environmentName
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionCreateCredentials,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypeAivenCredentials,
		ResourceName:    serviceType,
		EnvironmentName: &envName,
		TeamSlug:        &teamSlug,
		Data: AivenCredentialsActivityLogEntryData{
			ServiceType:  serviceType,
			InstanceName: instanceName,
			Permission:   permission,
			TTL:          ttl,
		},
	})
}
