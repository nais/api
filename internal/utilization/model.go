package utilization

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/nais/api/internal/slug"
)

type TeamUtilizationData struct {
	Requested       float64   `json:"requested"`
	Used            float64   `json:"used"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
}

type UtilizationResourceType string

const (
	UtilizationResourceTypeCPU    UtilizationResourceType = "CPU"
	UtilizationResourceTypeMemory UtilizationResourceType = "MEMORY"
)

var AllUtilizationResourceType = []UtilizationResourceType{
	UtilizationResourceTypeCPU,
	UtilizationResourceTypeMemory,
}

func (e UtilizationResourceType) IsValid() bool {
	switch e {
	case UtilizationResourceTypeCPU, UtilizationResourceTypeMemory:
		return true
	}
	return false
}

func (e UtilizationResourceType) String() string {
	return string(e)
}

func (e *UtilizationResourceType) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = UtilizationResourceType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid UtilizationResourceType", str)
	}
	return nil
}

func (e UtilizationResourceType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type UtilizationSample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Instance  string    `json:"instance"`
}

type WorkloadUtilization struct {
	EnvironmentName string       `json:"-"`
	WorkloadName    string       `json:"-"`
	TeamSlug        slug.Slug    `json:"-"`
	WorkloadType    WorkloadType `json:"-"`
}

type WorkloadUtilizationData struct {
	Requested float64 `json:"requested"`
	Used      float64 `json:"used"`

	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
	WorkloadName    string    `json:"-"`
}

type WorkloadUtilizationSeriesInput struct {
	Start        time.Time               `json:"start"`
	End          time.Time               `json:"end"`
	ResourceType UtilizationResourceType `json:"resourceType"`
}

func (w *WorkloadUtilizationSeriesInput) Step() int {
	diff := w.End.Sub(w.Start)

	switch {
	case diff <= time.Hour:
		return 18
	case diff <= 6*time.Hour:
		return 108
	case diff <= 24*time.Hour:
		return 432
	case diff <= 7*24*time.Hour:
		return 1008
	default:
		return 12960
	}
}

type WorkloadType int

const (
	WorkloadTypeApplication WorkloadType = iota
	WorkloadTypeJob
)

type TeamUtilization struct {
	TeamSlug slug.Slug `json:"-"`
}

type TeamServiceUtilization struct {
	TeamSlug slug.Slug `json:"-"`
}

type ApplicationInstanceUtilization struct {
	// Get the current usage for the requested resource type.
	Current float64 `json:"current"`
}

type WorkloadUtilizationRecommendations struct {
	client          ResourceUsageClient
	environmentName string
	workloadName    string
	teamSlug        slug.Slug
	start           time.Time
}

func (w WorkloadUtilizationRecommendations) CPURequestCores(ctx context.Context) (float64, error) {
	v, err := w.client.Query(ctx, w.environmentName, fmt.Sprintf(cpuRequestRecommendation, w.workloadName, w.teamSlug, w.start.Hour(), w.start.Add(time.Hour*12).Hour()))
	if err != nil {
		return 0, err
	}

	cpuReq := ensuredVal(v)

	return math.Max(cpuReq, minCPURequest), nil
}

func (w WorkloadUtilizationRecommendations) MemoryRequestBytes(ctx context.Context) (int64, error) {
	v, err := w.client.Query(ctx, w.environmentName, fmt.Sprintf(memoryRequestRecommendation, w.workloadName, w.teamSlug, w.start.Hour(), w.start.Add(time.Hour*12).Hour()))
	if err != nil {
		return 0, err
	}

	memReq := ensuredVal(v)
	return int64(math.Max(roundUpToNearest32MiB(memReq), minMemoryRequestBytes)), nil
}

func (w WorkloadUtilizationRecommendations) MemoryLimitBytes(ctx context.Context) (int64, error) {
	v, err := w.client.Query(ctx, w.environmentName, fmt.Sprintf(memoryLimitRecommendation, w.workloadName, w.teamSlug, w.start.Hour(), w.start.Add(time.Hour*12).Hour()))
	if err != nil {
		return 0, err
	}

	memLimit := ensuredVal(v)
	return int64(math.Max(roundUpToNearest32MiB(memLimit), minMemoryRequestBytes)), nil
}
