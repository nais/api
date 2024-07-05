package workload

import "github.com/nais/api/internal/graphv1/modelv1"

type Workload interface {
	modelv1.Node
	IsWorkload()
}
