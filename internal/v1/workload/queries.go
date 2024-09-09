package workload

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/ident"
)

func getImageByIdent(_ context.Context, id ident.Ident) (*ContainerImage, error) {
	name, err := parseImageIdent(id)
	if err != nil {
		return nil, err
	}

	name, tag, _ := strings.Cut(name, ":")
	return &ContainerImage{
		Name: name,
		Tag:  tag,
	}, nil
}
