package status

import (
	"context"
	"strings"

	"github.com/nais/api/internal/workload"
)

var allowedRegistries = []string{
	"europe-north1-docker.pkg.dev",
	"repo.adeo.no:5443",
	"oliver006/redis_exporter",
	"bitnami/redis",
	"docker.io/oliver006/redis_exporter",
	"docker.io/redis",
	"docker.io/bitnami/redis",
	"redis",
}

type checkDeprecatedRegsitry struct{}

func (c checkDeprecatedRegsitry) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	s := c.run(ctx, w)
	if s == nil {
		return nil, WorkloadStateNais
	}
	return []WorkloadStatusError{s}, WorkloadStateNotNais
}

func (checkDeprecatedRegsitry) run(_ context.Context, w workload.Workload) WorkloadStatusError {
	imageString := w.GetImageString()

	for _, registry := range allowedRegistries {
		if strings.HasPrefix(imageString, registry) {
			return nil
		}
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
		Level:      WorkloadStatusErrorLevelError,
		Registry:   registry,
		Name:       name,
		Tag:        tag,
		Repository: repository,
	}
}

func (checkDeprecatedRegsitry) Supports(w workload.Workload) bool {
	return true
}
