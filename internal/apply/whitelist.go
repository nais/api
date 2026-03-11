package apply

import "k8s.io/apimachinery/pkg/runtime/schema"

// AllowedResource identifies a Kubernetes resource by its apiVersion and kind.
type AllowedResource struct {
	APIVersion string
	Kind       string
}

// allowedResources is the single source of truth for which resources can be applied
// through the API. Each entry maps an apiVersion+kind pair to its GroupVersionResource,
// avoiding the need for a discovery client.
var allowedResources = map[AllowedResource]schema.GroupVersionResource{
	{APIVersion: "nais.io/v1alpha1", Kind: "Application"}: {
		Group: "nais.io", Version: "v1alpha1", Resource: "applications",
	},
	{APIVersion: "nais.io/v1", Kind: "Naisjob"}: {
		Group: "nais.io", Version: "v1", Resource: "naisjobs",
	},
}

// IsAllowed returns true if the given apiVersion and kind are in the whitelist.
func IsAllowed(apiVersion, kind string) bool {
	_, ok := allowedResources[AllowedResource{APIVersion: apiVersion, Kind: kind}]
	return ok
}

// GVRFor returns the GroupVersionResource for the given apiVersion and kind.
// The second return value is false if the resource is not in the whitelist.
func GVRFor(apiVersion, kind string) (schema.GroupVersionResource, bool) {
	gvr, ok := allowedResources[AllowedResource{APIVersion: apiVersion, Kind: kind}]
	return gvr, ok
}

// AllowedKinds returns a list of all allowed apiVersion+kind combinations.
// Useful for error messages.
func AllowedKinds() []AllowedResource {
	kinds := make([]AllowedResource, 0, len(allowedResources))
	for k := range allowedResources {
		kinds = append(kinds, k)
	}
	return kinds
}
