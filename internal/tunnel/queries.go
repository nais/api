package tunnel

import (
	"context"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	tunnelAPIVersion = "nais.io/v1alpha1"
	tunnelKind       = "Tunnel"
)

func Create(ctx context.Context, input CreateTunnelInput) (*CreateTunnelPayload, error) {
	if err := authz.CanCreateTunnel(ctx, slug.Slug(input.TeamSlug)); err != nil {
		return nil, err
	}

	addrs, err := net.LookupHost(input.TargetHost)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("DNS resolve %q: %w", input.TargetHost, err)
	}
	resolvedIP := addrs[0]

	namespace := input.TeamSlug
	loaders := FromContext(ctx)
	if loaders == nil {
		return nil, fmt.Errorf("tunnel loaders not found in context")
	}

	client, err := loaders.tunnelWatcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, namespace)
	if err != nil {
		return nil, err
	}

	tunnelName := fmt.Sprintf("tunnel-%s", uuid.NewString()[:8])

	res := &unstructured.Unstructured{}
	res.SetAPIVersion(tunnelAPIVersion)
	res.SetKind(tunnelKind)
	res.SetName(tunnelName)
	res.SetNamespace(namespace)

	res.Object["spec"] = map[string]interface{}{
		"teamSlug":        input.TeamSlug,
		"environment":     input.EnvironmentName,
		"clientPublicKey": input.ClientPublicKey,
		"target": map[string]interface{}{
			"host":       input.TargetHost,
			"port":       int64(input.TargetPort),
			"resolvedIP": resolvedIP,
		},
	}

	ret, err := client.Create(ctx, res, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("tunnel %q already exists", tunnelName)
		}
		return nil, err
	}

	t, err := converter(ret)
	if err != nil {
		return nil, err
	}
	t.Environment = input.EnvironmentName

	if err := LogTunnelCreated(ctx, t); err != nil {
		return nil, err
	}

	return &CreateTunnelPayload{Tunnel: t}, nil
}

func Get(ctx context.Context, id string) (*Tunnel, error) {
	loaders := FromContext(ctx)
	if loaders == nil {
		return nil, fmt.Errorf("tunnel loaders not in context")
	}
	for _, wrapped := range loaders.tunnelWatcher.All() {
		if wrapped.Obj.TunnelID == id {
			return wrapped.Obj, nil
		}
	}
	return nil, ErrTunnelNotFound
}

func UpdateSTUNEndpoint(ctx context.Context, id string, endpoint string) (*UpdateTunnelSTUNEndpointPayload, error) {
	loaders := FromContext(ctx)
	if loaders == nil {
		return nil, fmt.Errorf("tunnel loaders not found in context")
	}

	var found *Tunnel
	var envName string
	for _, w := range loaders.tunnelWatcher.All() {
		if w.Obj.TunnelID == id {
			found = w.Obj
			envName = w.Cluster
			break
		}
	}
	if found == nil {
		return nil, ErrTunnelNotFound
	}

	if err := authz.CanCreateTunnel(ctx, slug.Slug(found.TeamSlug)); err != nil {
		return nil, err
	}

	client, err := loaders.tunnelWatcher.ImpersonatedClientWithNamespace(ctx, envName, found.TeamSlug)
	if err != nil {
		return nil, err
	}

	existing, err := client.Get(ctx, found.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err := unstructured.SetNestedField(existing.Object, endpoint, "spec", "clientSTUNEndpoint"); err != nil {
		return nil, err
	}

	updated, err := client.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	t, err := converter(updated)
	if err != nil {
		return nil, err
	}
	t.Environment = envName

	return &UpdateTunnelSTUNEndpointPayload{Tunnel: t}, nil
}

func Delete(ctx context.Context, id string) error {
	loaders := FromContext(ctx)
	if loaders == nil {
		return fmt.Errorf("tunnel loaders not found in context")
	}

	var found *Tunnel
	var envName string
	for _, w := range loaders.tunnelWatcher.All() {
		if w.Obj.TunnelID == id {
			found = w.Obj
			envName = w.Cluster
			break
		}
	}
	if found == nil {
		return ErrTunnelNotFound
	}

	if err := authz.CanCreateTunnel(ctx, slug.Slug(found.TeamSlug)); err != nil {
		return err
	}

	if err := loaders.tunnelWatcher.Delete(ctx, envName, found.TeamSlug, found.Name); err != nil {
		return err
	}

	return LogTunnelDeleted(ctx, id, found.TeamSlug)
}
