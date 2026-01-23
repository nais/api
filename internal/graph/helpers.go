package graph

import (
	"context"

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
	preloads := getPreloads(ctx)
	if len(preloads) != 2 {
		return false
	}

	return preloads[0] == "pageInfo" && preloads[1] == "pageInfo.totalCount"
}
