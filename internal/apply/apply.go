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
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const maxBodySize = 5 * 1024 * 1024 // 5 MB

type Handler struct {
	dynamicClientFn DynamicClientFactory
	log             logrus.FieldLogger
}

type DynamicClientFactory func(environmentName string, teamSlug slug.Slug) (dynamic.Interface, error)

func NewHandler(dynamicClientFn DynamicClientFactory, log logrus.FieldLogger) *Handler {
	h := &Handler{
		log:             log,
		dynamicClientFn: dynamicClientFn,
	}

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	teamSlug := slug.Slug(r.PathValue("teamSlug"))
	environmentName := r.PathValue("environment")

	if err := authz.CanApplyKubernetesResource(ctx, teamSlug); err != nil {
		writeError(w, http.StatusForbidden, fmt.Sprintf("authorization failed: %s", err))
		return
	}

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

	var disallowed []string
	for i, res := range req.Resources {
		if !IsAllowed(res) {
			disallowed = append(disallowed, fmt.Sprintf("resources[%d]: %s/%s is not an allowed resource type", i, res.GetAPIVersion(), res.GetKind()))
		}
	}
	if len(disallowed) > 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("disallowed resource types: %s", strings.Join(disallowed, "; ")))
		return
	}

	results := make([]ResourceResult, 0, len(req.Resources))

	client, err := h.dynamicClientFn(environmentName, teamSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create client for environment %q: %s", environmentName, err))
		return
	}

	for _, res := range req.Resources {
		result := h.applyOne(ctx, client, teamSlug, environmentName, &res)
		results = append(results, result)
	}

	writeJSON(w, http.StatusOK, Response{Results: results})
}

func (h *Handler) applyOne(
	ctx context.Context,
	client dynamic.Interface,
	teamSlug slug.Slug,
	environmentName string,
	res *unstructured.Unstructured,
) ResourceResult {
	apiVersion := res.GetAPIVersion()
	kind := res.GetKind()
	name := res.GetName()
	resourceID := kind + "/" + name

	log := h.log.WithFields(logrus.Fields{
		"environment": environmentName,
		"team":        teamSlug,
		"name":        name,
		"kind":        kind,
	})

	if name == "" {
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environmentName,
			Status:          StatusError,
			Error:           "resource must have metadata.name",
		}
	}

	gvr, ok := GVRFor(apiVersion, kind)
	if !ok {
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environmentName,
			Status:          StatusError,
			Error:           fmt.Sprintf("no GVR mapping for %s/%s", apiVersion, kind),
		}
	}

	res.SetNamespace(string(teamSlug))

	applyResult, err := ApplyResource(ctx, client, gvr, res)
	if err != nil {
		log.WithError(err).Error("applying resource")
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environmentName,
			Status:          StatusError,
			Error:           fmt.Sprintf("apply failed: %s", err),
		}
	}

	var changes []activitylog.ResourceChangedField
	status := StatusCreated
	if !applyResult.Created {
		status = StatusApplied
		changes = Diff(applyResult.Before, applyResult.After)
	}

	if len(changes) == 0 && status == StatusApplied {
		// No changes were made, so we can skip creating an activity log entry.
		return ResourceResult{
			Resource:        resourceID,
			EnvironmentName: environmentName,
			Status:          StatusApplied,
		}
	}

	action := activitylog.ActivityLogEntryActionCreated
	if !applyResult.Created {
		action = activitylog.ActivityLogEntryActionUpdated
	}

	resourceType, _ := activitylog.ResourceTypeForKind(kind)

	logData := activitylog.GenericKubernetesResourceActivityLogEntryData{
		APIVersion:    apiVersion,
		Kind:          kind,
		ChangedFields: changes,
	}

	actor := authz.ActorFromContext(ctx)
	if ghActor, ok := actor.User.(*middleware.GitHubRepoActor); ok {
		claims := activitylog.GitHubActorClaims(ghActor.Claims)
		logData.GitHubActorClaims = &claims
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          action,
		Actor:           actor.User,
		ResourceType:    resourceType,
		ResourceName:    name,
		TeamSlug:        &teamSlug,
		EnvironmentName: &environmentName,
		Data:            logData,
	}); err != nil {
		log.WithError(err).Error("creating activity log entry")
	}

	return ResourceResult{
		Resource:        resourceID,
		EnvironmentName: environmentName,
		Status:          status,
		ChangedFields:   changes,
	}
}

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
