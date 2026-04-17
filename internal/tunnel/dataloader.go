package tunnel

import (
	"context"
	"time"

	"github.com/nais/api/internal/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var tunnelGVR = schema.GroupVersionResource{
	Group:    "nais.io",
	Version:  "v1alpha1",
	Resource: "tunnels",
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*Tunnel] {
	w := watcher.Watch(mgr, &Tunnel{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := converter(o)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(tunnelGVR))
	w.Start(ctx)
	return w
}

func converter(u *unstructured.Unstructured) (*Tunnel, error) {
	spec, _, _ := unstructured.NestedMap(u.Object, "spec")
	status, _, _ := unstructured.NestedMap(u.Object, "status")

	target, _, _ := unstructured.NestedMap(spec, "target")

	host, _, _ := unstructured.NestedString(target, "host")
	portFloat, _, _ := unstructured.NestedFloat64(target, "port")

	phase, _, _ := unstructured.NestedString(status, "phase")
	gatewayPublicKey, _, _ := unstructured.NestedString(status, "gatewayPublicKey")
	forwarderEndpoint, _, _ := unstructured.NestedString(status, "forwarderEndpoint")
	gatewayPodName, _, _ := unstructured.NestedString(status, "gatewayPodName")
	message, _, _ := unstructured.NestedString(status, "message")

	teamSlug, _, _ := unstructured.NestedString(spec, "teamSlug")
	environment, _, _ := unstructured.NestedString(spec, "environment")
	clientPublicKey, _, _ := unstructured.NestedString(spec, "clientPublicKey")

	createdAt := u.GetCreationTimestamp().Time
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	return &Tunnel{
		Name:              u.GetName(),
		TeamSlug:          teamSlug,
		Environment:       environment,
		Target:            Target{Host: host, Port: int32(portFloat)},
		ClientPublicKey:   clientPublicKey,
		GatewayPublicKey:  gatewayPublicKey,
		ForwarderEndpoint: forwarderEndpoint,
		GatewayPodName:    gatewayPodName,
		Phase:             Phase(phase),
		Message:           message,
		CreatedAt:         createdAt,
	}, nil
}
