package graph

import (
	"github.com/nais/api/internal/graph/model"
)

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
