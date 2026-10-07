package graph

import (
	"context"
	"maps"

	"github.com/99designs/gqlgen/graphql"
)

func getPreloads(ctx context.Context) []string {
	return getNestedPreloads(
		graphql.GetOperationContext(ctx),
		graphql.CollectFieldsCtx(ctx, nil),
		"",
	)
}

func getNestedPreloads(ctx *graphql.OperationContext, fields []graphql.CollectedField, prefix string) (preloads []string) {
	for _, column := range fields {
		prefixColumn := getPreloadString(prefix, column.Name)
		preloads = append(preloads, prefixColumn)
		preloads = append(preloads, getNestedPreloads(ctx, graphql.CollectFields(ctx, column.Selections, nil), prefixColumn)...)
	}
	return
}

func getPreloadString(prefix, name string) string {
	if len(prefix) > 0 {
		return prefix + "." + name
	}
	return name
}

func onlyAsksForPageInfoTotalCount(ctx context.Context) bool {
	preloads := make(map[string]struct{})
	for _, preload := range getPreloads(ctx) {
		switch preload {
		case "__typename", "pageInfo.__typename":
			continue
		default:
			preloads[preload] = struct{}{}
		}
	}

	return len(preloads) == 2 && maps.Equal(preloads, map[string]struct{}{
		"pageInfo":            {},
		"pageInfo.totalCount": {},
	})
}
