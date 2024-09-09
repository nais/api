package graphv1

import (
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
)

func (r *Resolver) ContainerImage() gengqlv1.ContainerImageResolver {
	return &containerImageResolver{r}
}

type containerImageResolver struct{ *Resolver }
