package graph

import (
	"testing"

	"github.com/nais/api/internal/graph/gengql"
)

func TestPostgresBranchMutationsRemoved(t *testing.T) {
	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()
	for _, tt := range []struct {
		mutation string
		typeName string
	}{
		{mutation: "createPostgresBranch", typeName: "CreatePostgresBranch"},
		{mutation: "activatePostgresBranch", typeName: "ActivatePostgresBranch"},
		{mutation: "deletePostgresBranch", typeName: "DeletePostgresBranch"},
	} {
		t.Run(tt.mutation, func(t *testing.T) {
			if schema.Mutation.Fields.ForName(tt.mutation) != nil {
				t.Errorf("removed mutation %s is still exposed", tt.mutation)
			}
			for _, suffix := range []string{"Input", "Payload"} {
				if schema.Types[tt.typeName+suffix] != nil {
					t.Errorf("removed type %s%s is still exposed", tt.typeName, suffix)
				}
			}
		})
	}
}

func TestPostgresSchemaRetainsDatabaseOperationsAndBranchHistory(t *testing.T) {
	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()
	for _, name := range []string{"createPostgres", "updatePostgres", "deletePostgres", "createPostgresAccess"} {
		if schema.Mutation.Fields.ForName(name) == nil {
			t.Errorf("mutation %s must remain available", name)
		}
	}
	for _, name := range []string{
		"PostgresBranch",
		"PostgresBranchCreatedActivityLogEntry",
		"PostgresBranchActivatedActivityLogEntry",
		"PostgresBranchDeletedActivityLogEntry",
	} {
		if schema.Types[name] == nil {
			t.Errorf("type %s must remain available", name)
		}
	}
}
