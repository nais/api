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
// The clusterConfigs map is used to validate that an environment name exists.
// The clientFactory is used to create dynamic Kubernetes clients per environment.
func Handler(clusterConfigs kubernetes.ClusterConfigMap, clientFactory DynamicClientFactory, log logrus.FieldLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		actor := authz.ActorFromContext(ctx)
		if actor == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

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

		environmentParam := r.URL.Query().Get("environment")

		// Phase 1: Validate all resources before applying any.
		// Check that all resources are allowed kinds and have valid environment targets.
		var disallowed []string
		for i, res := range req.Resources {
			apiVersion := res.GetAPIVersion()
			kind := res.GetKind()
			if !IsAllowed(apiVersion, kind) {
				disallowed = append(disallowed, fmt.Sprintf("resources[%d]: %s/%s is not an allowed resource type", i, apiVersion, kind))
			}
		}
		if len(disallowed) > 0 {
			allowed := AllowedKinds()
			allowedStrs := make([]string, len(allowed))
			for i, a := range allowed {
				allowedStrs[i] = a.APIVersion + "/" + a.Kind
			}
			writeError(w, http.StatusBadRequest, fmt.Sprintf(
				"disallowed resource types: %s. Allowed types: %s",
				strings.Join(disallowed, "; "),
				strings.Join(allowedStrs, ", "),
			))
			return
		}

		// Phase 2: Apply each resource, collecting results.
		results := make([]ResourceResult, 0, len(req.Resources))
		hasErrors := false

		for _, res := range req.Resources {
			result := applyOne(ctx, clusterConfigs, clientFactory, environmentParam, &res, actor, log)
			if result.Status == StatusError {
				hasErrors = true
			}
			results = append(results, result)
		}

		resp := Response{Results: results}

		// Determine HTTP status code.
		statusCode := http.StatusOK
		if hasErrors {
			statusCode = http.StatusMultiStatus
		}

		writeJSON(w, statusCode, resp)
	}
}

// applyOne processes a single resource: resolves environment, authorizes, applies, diffs, and logs.
func applyOne(
	ctx context.Context,
	clusterConfigs kubernetes.ClusterConfigMap,
	clientFactory DynamicClientFactory,
	environmentParam string,
	res *unstructured.Unstructured,
	actor *authz.Actor,
	log logrus.FieldLogger,
) ResourceResult {
	apiVersion := res.GetAPIVersion()
	kind := res.GetKind()
	name := res.GetName()
	namespace := res.GetNamespace()
	resourceID := kind + "/" + name

	// Resolve environment: annotation takes precedence over query parameter.
	environment := environmentParam
	if ann := res.GetAnnotations(); ann != nil {
		if e, ok := ann["nais.io/environment"]; ok && e != "" {
			environment = e
		}
	}

	if environment == "" {
		return ResourceResult{
			Resource:  resourceID,
			Namespace: namespace,
			Status:    StatusError,
			Error:     "no environment specified (use ?environment= query parameter or nais.io/environment annotation)",
		}
	}

	// Validate environment exists.
	if _, ok := clusterConfigs[environment]; !ok {
		return ResourceResult{
			Resource:    resourceID,
			Namespace:   namespace,
			Environment: environment,
			Status:      StatusError,
			Error:       fmt.Sprintf("unknown environment: %q", environment),
		}
	}

	// Validate resource has name and namespace.
	if name == "" {
		return ResourceResult{
			Resource:    resourceID,
			Namespace:   namespace,
			Environment: environment,
			Status:      StatusError,
			Error:       "resource must have metadata.name",
		}
	}
	if namespace == "" {
		return ResourceResult{
			Resource:    resourceID,
			Environment: environment,
			Status:      StatusError,
			Error:       "resource must have metadata.namespace",
		}
	}

	// Authorize: derive team slug from namespace.
	teamSlug := slug.Slug(namespace)
	if err := authorizeResource(ctx, kind, teamSlug); err != nil {
		return ResourceResult{
			Resource:    resourceID,
			Namespace:   namespace,
			Environment: environment,
			Status:      StatusError,
			Error:       fmt.Sprintf("authorization failed: %s", err),
		}
	}

	// Resolve GVR.
	gvr, ok := GVRFor(apiVersion, kind)
	if !ok {
		return ResourceResult{
			Resource:    resourceID,
			Namespace:   namespace,
			Environment: environment,
			Status:      StatusError,
			Error:       fmt.Sprintf("no GVR mapping for %s/%s", apiVersion, kind),
		}
	}

	// Create dynamic client for environment.
	client, err := clientFactory(ctx, environment)
	if err != nil {
		log.WithError(err).WithField("environment", environment).Error("creating dynamic client")
		return ResourceResult{
			Resource:    resourceID,
			Namespace:   namespace,
			Environment: environment,
			Status:      StatusError,
			Error:       fmt.Sprintf("failed to create client for environment %q: %s", environment, err),
		}
	}

	// Apply the resource.
	applyResult, err := ApplyResource(ctx, client, gvr, res)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"environment": environment,
			"namespace":   namespace,
			"name":        name,
			"kind":        kind,
		}).Error("applying resource")
		return ResourceResult{
			Resource:    resourceID,
			Namespace:   namespace,
			Environment: environment,
			Status:      StatusError,
			Error:       fmt.Sprintf("apply failed: %s", err),
		}
	}

	// Diff before and after.
	var changes []FieldChange
	status := StatusCreated
	if !applyResult.Created {
		status = StatusApplied
		changes = Diff(applyResult.Before, applyResult.After)
	}

	// Write activity log entry.
	action := ActivityLogEntryActionCreated
	if !applyResult.Created {
		action = ActivityLogEntryActionApplied
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          action,
		Actor:           actor.User,
		ResourceType:    ActivityLogEntryResourceTypeApply,
		ResourceName:    name,
		TeamSlug:        &teamSlug,
		EnvironmentName: &environment,
		Data: ApplyActivityLogEntryData{
			APIVersion:    apiVersion,
			Kind:          kind,
			ChangedFields: changes,
		},
	}); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"environment": environment,
			"namespace":   namespace,
			"name":        name,
			"kind":        kind,
		}).Error("creating activity log entry")
		// Don't fail the apply because of a logging error.
	}

	return ResourceResult{
		Resource:      resourceID,
		Namespace:     namespace,
		Environment:   environment,
		Status:        status,
		ChangedFields: changes,
	}
}

// authorizeResource checks if the current actor is authorized to apply the given kind
// to the team derived from the resource namespace.
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
