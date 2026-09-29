package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes/watcher"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// All connection resources are read on demand after the caller has been
// authorized as the PostgresAccess owner. No credentials enter the watch cache.
func loadPostgresAccessConnection(ctx context.Context, access *unstructured.Unstructured, input PostgresAccessConnectionInput, connection *PostgresAccessConnectionDetails, tokenSecretName string) error {
	if access.GetUID() == "" {
		return apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	namespace := input.TeamSlug.String()
	tokenSecret, err := getAccessResource(ctx, input.EnvironmentName, namespace, tokenSecretName, schema.GroupVersionResource{Version: "v1", Resource: "secrets"})
	if err != nil {
		return err
	}
	token, err := accessSecretData(tokenSecret, access, "token")
	if err != nil {
		return err
	}
	// The broker creates short names (postgres-access-<uuid>), so this is the
	// pgrator-owned credential Secret for this access, not a client-supplied name.
	credential, err := getAccessResource(ctx, input.EnvironmentName, namespace, access.GetName()+"-credentials", schema.GroupVersionResource{Version: "v1", Resource: "secrets"})
	if err != nil {
		return err
	}
	password, err := accessSecretData(credential, access, corev1.BasicAuthPasswordKey)
	if err != nil {
		return err
	}
	username, err := accessSecretData(credential, access, corev1.BasicAuthUsernameKey)
	if err != nil {
		return err
	}
	role, _, err := unstructured.NestedString(access.Object, "status", "databaseRole")
	if err != nil || role != username {
		return apierror.Errorf("credentials for PostgresAccess %q are not available", access.GetName())
	}

	serverCA, _, err := unstructured.NestedString(access.Object, "status", "serverCASecret")
	if err != nil || serverCA == "" {
		return apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	caSecret, err := getAccessResource(ctx, input.EnvironmentName, namespace, serverCA, schema.GroupVersionResource{Version: "v1", Resource: "secrets"})
	if err != nil {
		return err
	}
	var ca corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(caSecret.Object, &ca); err != nil {
		return fmt.Errorf("reading server CA for PostgresAccess %q: %w", access.GetName(), err)
	}
	if len(ca.Data["ca.crt"]) == 0 {
		return apierror.Errorf("server CA for PostgresAccess %q is not available", access.GetName())
	}

	connection.Username = username
	connection.Password = password
	connection.CACertificate = string(ca.Data["ca.crt"])
	serverName, _, err := unstructured.NestedString(access.Object, "status", "serverName")
	if err != nil || serverName == "" {
		return apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	connection.ServerName = serverName
	connection.RelayEndpoint = fmt.Sprintf("https://relay.external.%s.%s.cloud.nais.io:8443", input.EnvironmentName, fromContext(ctx).tenantName)
	relayName, _, err := unstructured.NestedString(access.Object, "status", "relayAccess")
	if err != nil || relayName == "" {
		return apierror.Errorf("PostgresAccess %q is not ready", access.GetName())
	}
	connection.RelayAccess = namespace + "/" + relayName
	connection.RelayToken = token
	return nil
}

func accessSecretData(secret, access *unstructured.Unstructured, key string) (string, error) {
	if !metav1.IsControlledBy(secret, access) || secret.GetDeletionTimestamp() != nil {
		return "", apierror.Errorf("credentials for PostgresAccess %q are not available", access.GetName())
	}
	var typed corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(secret.Object, &typed); err != nil {
		return "", fmt.Errorf("reading credentials for PostgresAccess %q: %w", access.GetName(), err)
	}
	value := string(typed.Data[key])
	if strings.TrimSpace(value) == "" {
		return "", apierror.Errorf("credentials for PostgresAccess %q are incomplete", access.GetName())
	}
	return value, nil
}

func getAccessResource(ctx context.Context, environment, namespace, name string, gvr schema.GroupVersionResource) (*unstructured.Unstructured, error) {
	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, environment, watcher.WithImpersonatedClientGVR(gvr))
	if err != nil {
		return nil, fmt.Errorf("creating %s client: %w", gvr.Resource, err)
	}
	resource, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("%s for PostgresAccess is not available", gvr.Resource)
	}
	if err != nil {
		return nil, fmt.Errorf("getting %s for PostgresAccess: %w", gvr.Resource, err)
	}
	return resource, nil
}
