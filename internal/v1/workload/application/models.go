package application

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type (
	ApplicationConnection = pagination.Connection[*Application]
	ApplicationEdge       = pagination.Edge[*Application]
)

type Application struct {
	workload.Base
	Resources *ApplicationResources `json:"resources"`
}

func (Application) IsWorkload() {}
func (Application) IsNode()     {}

func (a Application) ID() ident.Ident {
	return newIdent(a.TeamSlug, a.EnvironmentName, a.Name)
}

type ApplicationOrder struct {
	Field     ApplicationOrderField  `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type ApplicationOrderField string

const (
	ApplicationOrderFieldName           ApplicationOrderField = "NAME"
	ApplicationOrderFieldStatus         ApplicationOrderField = "STATUS"
	ApplicationOrderFieldEnvironment    ApplicationOrderField = "ENVIRONMENT"
	ApplicationOrderFieldDeploymentTime ApplicationOrderField = "DEPLOYMENT_TIME"
)

func (e ApplicationOrderField) IsValid() bool {
	switch e {
	case ApplicationOrderFieldStatus, ApplicationOrderFieldName, ApplicationOrderFieldEnvironment, ApplicationOrderFieldDeploymentTime:
		return true
	}
	return false
}

func (e ApplicationOrderField) String() string {
	return string(e)
}

func (e *ApplicationOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ApplicationOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ApplicationOrderField", str)
	}
	return nil
}

func (e ApplicationOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ScalingStrategy interface {
	IsScalingStrategy()
}

type ApplicationResources struct {
	Limits   *workload.WorkloadResourceQuantity `json:"limits"`
	Requests *workload.WorkloadResourceQuantity `json:"requests"`
	Scaling  *ApplicationScaling                `json:"scaling"`
}

func (ApplicationResources) IsWorkloadResources() {}

type ApplicationScaling struct {
	MinInstances int               `json:"minInstances"`
	MaxInstances int               `json:"maxInstances"`
	Strategies   []ScalingStrategy `json:"strategies"`
}

type CPUScalingStrategy struct {
	Threshold int `json:"threshold"`
}

func (CPUScalingStrategy) IsScalingStrategy() {}

type KafkaLagScalingStrategy struct {
	Threshold     int    `json:"threshold"`
	ConsumerGroup string `json:"consumerGroup"`
	Topic         string `json:"topic"`
}

func (KafkaLagScalingStrategy) IsScalingStrategy() {}

func toGraphApplicationResources(spec nais_io_v1alpha1.ApplicationSpec) *ApplicationResources {
	ret := &ApplicationResources{
		Limits:   &workload.WorkloadResourceQuantity{},
		Requests: &workload.WorkloadResourceQuantity{},
		Scaling:  &ApplicationScaling{},
	}

	if q, err := resource.ParseQuantity(spec.Resources.Limits.Cpu); err == nil {
		ret.Limits.CPU = q.AsApproximateFloat64()
	}

	if m, err := resource.ParseQuantity(spec.Resources.Limits.Memory); err == nil {
		ret.Limits.Memory = m.Value()
	}

	if q, err := resource.ParseQuantity(spec.Resources.Requests.Cpu); err == nil {
		ret.Requests.CPU = q.AsApproximateFloat64()
	}

	if m, err := resource.ParseQuantity(spec.Resources.Requests.Memory); err == nil {
		ret.Requests.Memory = m.Value()
	}

	replicas := spec.Replicas
	if replicas != nil {
		if replicas.Min != nil {
			ret.Scaling.MinInstances = *replicas.Min
		}

		if replicas.Max != nil {
			ret.Scaling.MaxInstances = *replicas.Max
		}

		strategy := replicas.ScalingStrategy
		if strategy != nil && strategy.Cpu != nil && strategy.Cpu.ThresholdPercentage > 0 {
			ret.Scaling.Strategies = append(ret.Scaling.Strategies, CPUScalingStrategy{
				Threshold: strategy.Cpu.ThresholdPercentage,
			})
		}

		if strategy != nil && strategy.Kafka != nil && strategy.Kafka.Threshold > 0 {
			ret.Scaling.Strategies = append(ret.Scaling.Strategies, KafkaLagScalingStrategy{
				Threshold:     strategy.Kafka.Threshold,
				ConsumerGroup: strategy.Kafka.ConsumerGroup,
				Topic:         strategy.Kafka.Topic,
			})
		}
	}

	return ret
}

func toGraphApplication(application *nais_io_v1alpha1.Application, environmentName string) *Application {
	return &Application{
		Base: workload.Base{
			Name:            application.Name,
			EnvironmentName: environmentName,
			TeamSlug:        slug.Slug(application.Namespace),
			ImageString:     application.Spec.Image,
		},
		Resources: toGraphApplicationResources(application.Spec),
	}
}
