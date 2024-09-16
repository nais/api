package workload

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/ident"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
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

func GetMaskinPortenAuthIntegration(mp *nais_io_v1.Maskinporten) *MaskinportenAuthIntegration {
	if mp == nil || !mp.Enabled {
		return nil
	}

	return &MaskinportenAuthIntegration{}
}

func GetTokenXAuthIntegration(tx *nais_io_v1.TokenX) *TokenXAuthIntegration {
	if tx == nil || !tx.Enabled {
		return nil
	}

	return &TokenXAuthIntegration{}
}

func GetIDPortenAuthIntegration(idp *nais_io_v1.IDPorten) *IDPortenAuthIntegration {
	if idp == nil || !idp.Enabled {
		return nil
	}

	return &IDPortenAuthIntegration{}
}

func GetEntraIDAuthIntegrationForApplication(azure *nais_io_v1.Azure) *EntraIDAuthIntegration {
	if azure == nil || azure.Application == nil || !azure.Application.Enabled {
		return nil
	}

	return &EntraIDAuthIntegration{}
}

func GetEntraIDAuthIntegrationForJob(azure *nais_io_v1.AzureNaisJob) *EntraIDAuthIntegration {
	if azure == nil || azure.Application == nil || !azure.Application.Enabled {
		return nil
	}

	return &EntraIDAuthIntegration{}
}
