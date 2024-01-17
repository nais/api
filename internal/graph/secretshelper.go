package graph

import (
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func makeSecretIdent(env, namespace, name string) scalar.Ident {
	return scalar.SecretIdent("secret_" + env + "_" + namespace + "_" + name)
}

func convertSecretData(data []*model.SecretTupleInput) map[string]string {
	ret := make(map[string]string, len(data))
	for _, value := range data {
		ret[value.Key] = value.Value
	}
	return ret
}

func emptySecret(name string, team slug.Slug, env string) *model.Secret {
	return &model.Secret{
		ID:   makeSecretIdent(env, team.String(), name),
		Name: name,
		Data: make(map[string]string),
	}
}

func convertSecretDataToTuple(data map[string]string) []*model.SecretTuple {
	ret := make([]*model.SecretTuple, 0, len(data))
	for key, value := range data {
		ret = append(ret, &model.SecretTuple{
			Key:   key,
			Value: value,
		})
	}
	return ret
}
