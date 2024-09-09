package workload

import (
	"fmt"

	"github.com/nais/api/internal/v1/graphv1/ident"
)

type identType int

const (
	identKeyImg identType = iota
)

func init() {
	ident.RegisterIdentType(identKeyImg, "IMG", getImageByIdent)
}

func newImageIdent(imgString string) ident.Ident {
	return ident.NewIdent(identKeyImg, imgString)
}

func parseImageIdent(id ident.Ident) (string, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid image ident")
	}

	return parts[0], nil
}
