package status

import (
	"context"
	"strings"

	"github.com/nais/api/internal/workload"
)

type checkDeprecatedRegsitry struct{}

func (c checkDeprecatedRegsitry) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	s := c.run(ctx, w)
	if s == nil {
		return nil, WorkloadStateNais
	}
	return []WorkloadStatusError{s}, WorkloadStateNais
}

func (checkDeprecatedRegsitry) run(_ context.Context, w workload.Workload) WorkloadStatusError {
	imageString := w.GetImageString()

	if strings.Contains(imageString, "europe-north1-docker.pkg.dev") {
		return nil
	}

	parts := strings.Split(imageString, ":")
	tag := "unknown"
	if len(parts) > 1 {
		tag = parts[1]
	}
	parts = strings.Split(parts[0], "/")
	registry := parts[0]
	name := parts[len(parts)-1]
	repository := ""
	if len(parts) > 2 {
		repository = strings.Join(parts[1:len(parts)-1], "/")
	}
	return &WorkloadStatusDeprecatedRegistry{
		Level:      WorkloadStatusErrorLevelWarning,
		Registry:   registry,
		Name:       name,
		Tag:        tag,
		Repository: repository,
	}
}

func (checkDeprecatedRegsitry) Supports(w workload.Workload) bool {
	return true
}
