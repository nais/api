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

func toGraphApplication(a *nais_io_v1alpha1.Application, environmentName string) *Application {
	r := &ApplicationResources{
		Limits:   &workload.WorkloadResourceQuantity{},
		Requests: &workload.WorkloadResourceQuantity{},
		Scaling:  &ApplicationScaling{},
	}

	q, err := resource.ParseQuantity(a.Spec.Resources.Limits.Cpu)
	if err == nil {
		r.Limits.CPU = q.AsApproximateFloat64()
	}

	m, err := resource.ParseQuantity(a.Spec.Resources.Limits.Memory)
	if err == nil {
		r.Limits.Memory = m.Value()
	}

	q, err = resource.ParseQuantity(a.Spec.Resources.Requests.Cpu)
	if err == nil {
		r.Requests.CPU = q.AsApproximateFloat64()
	}

	m, err = resource.ParseQuantity(a.Spec.Resources.Requests.Memory)
	if err == nil {
		r.Requests.Memory = m.Value()
	}

	if a.Spec.Replicas != nil {
		if a.Spec.Replicas.Min != nil {
			r.Scaling.MinInstances = *a.Spec.Replicas.Min
		}
		if a.Spec.Replicas.Max != nil {
			r.Scaling.MaxInstances = *a.Spec.Replicas.Max
		}

		if a.Spec.Replicas.ScalingStrategy != nil && a.Spec.Replicas.ScalingStrategy.Cpu != nil && a.Spec.Replicas.ScalingStrategy.Cpu.ThresholdPercentage > 0 {
			r.Scaling.Strategies = append(r.Scaling.Strategies, CPUScalingStrategy{
				Threshold: a.Spec.Replicas.ScalingStrategy.Cpu.ThresholdPercentage,
			})
		}

		if a.Spec.Replicas.ScalingStrategy != nil && a.Spec.Replicas.ScalingStrategy.Kafka != nil && a.Spec.Replicas.ScalingStrategy.Kafka.Threshold > 0 {
			r.Scaling.Strategies = append(r.Scaling.Strategies, KafkaLagScalingStrategy{
				Threshold:     a.Spec.Replicas.ScalingStrategy.Kafka.Threshold,
				ConsumerGroup: a.Spec.Replicas.ScalingStrategy.Kafka.ConsumerGroup,
				Topic:         a.Spec.Replicas.ScalingStrategy.Kafka.Topic,
			})
		}
	}

	return &Application{
		Base: workload.Base{
			Name:            a.Name,
			EnvironmentName: environmentName,
			TeamSlug:        slug.Slug(a.Namespace),
			ImageString:     a.Spec.Image,
		},
		Resources: r,
	}
}
