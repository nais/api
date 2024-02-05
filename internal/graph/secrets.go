package graph

import (
	"cmp"
	"slices"

	"github.com/nais/api/internal/graph/model"
)

func convertSecretDataToTuple(data map[string]string) []*model.Variable {
	ret := make([]*model.Variable, 0, len(data))
	for key, value := range data {
		ret = append(ret, &model.Variable{
			Name:  key,
			Value: value,
		})
	}
	slices.SortFunc(ret, func(a, b *model.Variable) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return ret
}
