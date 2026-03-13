package apply

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const maxBodySize = 5 * 1024 * 1024 // 5 MB

// DynamicClientFactory creates a dynamic Kubernetes client for a given environment.
// In production this creates an impersonated client from environment configs.
// In tests this can return fake dynamic clients.
type DynamicClientFactory func(ctx context.Context, environment string) (dynamic.Interface, error)

// NewImpersonatingClientFactory returns a DynamicClientFactory that creates
// impersonated dynamic clients from the provided environment config map.
func NewImpersonatingClientFactory(clusterConfigs kubernetes.ClusterConfigMap) DynamicClientFactory {
	return func(ctx context.Context, environment string) (dynamic.Interface, error) {
		return ImpersonatedClient(ctx, clusterConfigs, environment)
	}
}

// Handler returns an http.HandlerFunc that handles apply requests.
// It validates the request body, checks authorization, applies resources
// to Kubernetes environments via server-side apply, diffs the results, and
// writes activity log entries.
//
// The team slug and environment are read from URL path parameters.
// The clusterConfigs map is used to validate that an environment name exists.
// The clientFactory is used to create dynamic Kubernetes clients per environment.
func Handler(clusterConfigs kubernetes.ClusterConfigMap, clientFactory DynamicClientFactory, log logrus.FieldLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		teamSlug := slug.Slug(r.PathValue("teamSlug"))
		environment := r.PathValue("environment")

		actor := authz.ActorFromContext(ctx)

		// Read and parse request body.
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "unable to read request body")
			return
		}
		if len(body) > maxBodySize {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("request body too large (max %d bytes)", maxBodySize))
			return
		}

		var req request
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		if len(req.Resources) == 0 {
			writeError(w, http.StatusBadRequest, "no resources provided")
			return
		}

		// Phase 1: Validate all resources before applying any.
		var disallowed []string
		for i, res := range req.Resources {
			apiVersion := res.GetAPIVersion()
			kind := res.GetKind()
			if !IsAllowed(apiVersion, kind) {
				disallowed = append(disallowed, fmt.Sprintf("resources[%d]: %s/%s is not an allowed resource type", i, apiVersion, kind))
			}
		}
		if len(disallowed) > 0 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("disallowed resource types: %s", strings.Join(disallowed, "; ")))
			return
		}

		// Phase 2: Apply each resource, collecting results.
		results := make([]ResourceResult, 0, len(req.Resources))

		for _, res := range req.Resources {
			result := applyOne(ctx, clusterConfigs, clientFactory, teamSlug, environment, &res, actor, log)
			results = append(results, result)
		}

		writeJSON(w, http.StatusOK, Response{Results: results})
	}
}

// applyOne processes a single resource: authorizes, applies, diffs, and logs.
func applyOne(
	ctx context.Context,
	clusterConfigs kubernetes.ClusterConfigMap,
	clientFactory DynamicClientFactory,
	teamSlug slug.Slug,
	environment string,
	res *unstructured.Unstructured,
	actor *authz.Actor,
	log logrus.FieldLogger,
) ResourceResult {
	apiVersion := res.GetAPIVersion()
	kind := res.GetKind()
	name := res.GetName()
	resourceID := kind + "/" + name

	log = log.WithFields(logrus.Fields{
		"environment": environment,
		"team":        teamSlug,
		"name":        name,
		"kind":        kind,
	})

	// Validate environment exists.
	if _, ok := clusterConfigs[environment]; !ok {
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environment,
			Status:          StatusError,
			Error:           fmt.Sprintf("unknown environment: %q", environment),
		}
	}

	// Validate resource has a name.
	if name == "" {
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environment,
			Status:          StatusError,
			Error:           "resource must have metadata.name",
		}
	}

	// Authorize the actor for this team and kind.
	if err := authorizeResource(ctx, kind, teamSlug); err != nil {
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environment,
			Status:          StatusError,
			Error:           fmt.Sprintf("authorization failed: %s", err),
		}
	}

	// Resolve GVR.
	gvr, ok := GVRFor(apiVersion, kind)
	if !ok {
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environment,
			Status:          StatusError,
			Error:           fmt.Sprintf("no GVR mapping for %s/%s", apiVersion, kind),
		}
	}

	// Force the namespace to match the team slug from the URL.
	res.SetNamespace(string(teamSlug))

	// Create dynamic client for environment.
	client, err := clientFactory(ctx, environment)
	if err != nil {
		log.WithError(err).Error("creating dynamic client")
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environment,
			Status:          StatusError,
			Error:           fmt.Sprintf("failed to create client for environment %q: %s", environment, err),
		}
	}

	// Apply the resource.
	applyResult, err := ApplyResource(ctx, client, gvr, res)
	if err != nil {
		log.WithError(err).Error("applying resource")
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environment,
			Status:          StatusError,
			Error:           fmt.Sprintf("apply failed: %s", err),
		}
	}

	// Diff before and after.
	var changes []activitylog.ResourceChangedField
	status := StatusCreated
	if !applyResult.Created {
		status = StatusApplied
		changes = Diff(applyResult.Before, applyResult.After)
	}

	// Write activity log entry.
	action := activitylog.ActivityLogEntryActionCreated
	if !applyResult.Created {
		action = activitylog.ActivityLogEntryActionUpdated
	}

	resourceType, _ := activitylog.ResourceTypeForKind(kind)
	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          action,
		Actor:           actor.User,
		ResourceType:    resourceType,
		ResourceName:    name,
		TeamSlug:        &teamSlug,
		EnvironmentName: &environment,
		Data: activitylog.GenericKubernetesResourceActivityLogEntryData{
			APIVersion:    apiVersion,
			Kind:          kind,
			ChangedFields: changes,
		},
	}); err != nil {
		log.WithError(err).Error("creating activity log entry")
		// Don't fail the apply because of a logging error.
	}

	return ResourceResult{
		Resource:        resourceID,
		EnvironmentName: environment,
		Status:          status,
		ChangedFields:   changes,
	}
}

// authorizeResource checks if the current actor is authorized to apply the given kind
// to the team from the URL.
func authorizeResource(ctx context.Context, kind string, teamSlug slug.Slug) error {
	switch kind {
	case "Application":
		return authz.CanUpdateApplications(ctx, teamSlug)
	case "Naisjob":
		return authz.CanUpdateJobs(ctx, teamSlug)
	default:
		return fmt.Errorf("no authorization mapping for kind %q", kind)
	}
}

// request is the incoming JSON request body.
type request struct {
	Resources []unstructured.Unstructured `json:"resources"`
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}
