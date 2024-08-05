package workload

import "github.com/nais/api/internal/v1/graphv1/modelv1"

type Workload interface {
	modelv1.Node
	IsWorkload()
}
