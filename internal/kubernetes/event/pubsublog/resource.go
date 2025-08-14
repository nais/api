package pubsublog

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

type Resource struct {
	GVR       *schema.GroupVersionResource
	Namespace string
	Name      string
	Extra     string
}

func (r Resource) Empty() bool {
	return r.GVR == nil
}

// parseResourceName parses a resource name string into a Resource struct.
// The expected format is "group/version/namespaces/namespace/kind/name(/extra)"
func parseResourceName(s string) (Resource, error) {
	parts := strings.Split(s, "/")
	if len(parts) < 5 {
		return Resource{}, fmt.Errorf("invalid resource name format: %s", s)
	}

	gv, err := schema.ParseGroupVersion(parts[0] + "/" + parts[1])
	if err != nil {
		return Resource{}, fmt.Errorf("parsing group/version: %w", err)
	}

	if gv.Group == "core" {
		gv.Group = "" // Handle core group as empty string
	}

	resource := Resource{
		GVR:       ptr.To(gv.WithResource(parts[4])),
		Namespace: parts[3],
		Name:      parts[5],
	}
	if len(parts) > 6 {
		resource.Extra = strings.Join(parts[6:], "/")
	}

	return resource, nil
}
