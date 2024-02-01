package graph

import (
	"cmp"
	"slices"

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
	slices.SortFunc(ret, func(a, b *model.SecretTuple) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return ret
}
