package activitylog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware/github"
)

type claimsActor struct {
	authz.AuthenticatedUser
	claims *github.GitHubActorClaims
}

func (a claimsActor) Identity() string { return "github-repo:nais/example" }

func (a claimsActor) GitHubActorClaims() *github.GitHubActorClaims { return a.claims }

type captureActivityLogDB struct {
	activitylogsql.DBTX
	args []any
}

func (db *captureActivityLogDB) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	db.args = args
	return pgconn.CommandTag{}, nil
}

func TestCreateGitHubActorClaimsColumn(t *testing.T) {
	claims := &github.GitHubActorClaims{Actor: "octocat", Repository: "nais/example", RunID: "123"}
	for _, test := range []struct {
		name string
		data any
	}{
		{name: "without data"},
		{name: "with data", data: &GenericKubernetesResourceActivityLogEntryData{Kind: "Application", GitHubActorClaims: claims}},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := &captureActivityLogDB{}
			ctx := context.WithValue(context.Background(), loadersKey, &loaders{internalQuerier: activitylogsql.New(db)})
			if err := Create(ctx, CreateInput{Actor: claimsActor{claims: claims}, Data: test.data}); err != nil {
				t.Fatal(err)
			}
			if len(db.args) != 8 {
				t.Fatalf("expected 8 arguments, got %d", len(db.args))
			}
			if data := db.args[6].([]byte); test.data == nil {
				if data != nil {
					t.Fatalf("expected no data, got %s", data)
				}
			} else {
				var payload map[string]json.RawMessage
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatal(err)
				}
				if _, ok := payload["gitHubActorClaims"]; ok {
					t.Fatalf("claims were stored in data: %s", data)
				}
			}
			var stored github.GitHubActorClaims
			if err := json.Unmarshal(db.args[7].([]byte), &stored); err != nil {
				t.Fatal(err)
			}
			if stored.Actor != claims.Actor || stored.RunID != claims.RunID {
				t.Fatalf("unexpected stored claims: %+v", stored)
			}
		})
	}
}

func TestGitHubActorClaimsNotInActivityLogData(t *testing.T) {
	claims := &github.GitHubActorClaims{Actor: "octocat", Repository: "nais/example", RunID: "123"}
	for _, test := range []struct {
		name string
		data any
	}{
		{name: "entry without data"},
		{name: "entry with data", data: &GenericKubernetesResourceActivityLogEntryData{
			Kind:              "Application",
			GitHubActorClaims: claims,
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := MarshalData(CreateInput{Data: test.data})
			if err != nil {
				t.Fatal(err)
			}
			if test.data == nil {
				if data != nil {
					t.Fatalf("expected no data, got %s", data)
				}
				return
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if _, ok := payload["gitHubActorClaims"]; ok {
				t.Fatalf("claims must not be stored in data: %s", data)
			}
			if _, ok := payload["kind"]; !ok {
				t.Fatalf("existing payload was lost: %s", data)
			}
		})
	}
}

func TestActivityLogEntryGitHubActorClaims(t *testing.T) {
	data := []byte(`{"apiVersion":"v1","kind":"ConfigMap"}`)
	claims := []byte(`{"actor":"octocat","repository":"nais/example","run_id":"123"}`)
	for _, test := range []struct {
		name       string
		claims     []byte
		data       []byte
		wantClaims bool
	}{
		{name: "repository actor", claims: claims, data: data, wantClaims: true},
		{name: "column takes precedence over legacy data", claims: claims, data: []byte(`{"kind":"ConfigMap","gitHubActorClaims":{"actor":"old"}}`), wantClaims: true},
		{name: "legacy entry", data: []byte(`{"apiVersion":"v1","kind":"ConfigMap","gitHubActorClaims":{"actor":"octocat","run_id":"123"}}`), wantClaims: true},
		{name: "entry without claims", data: data},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry, err := toGraphActivityLogEntry(&activitylogsql.ActivityLogCombinedView{
				Actor:             "github-repo:nais/example",
				Action:            string(ActivityLogEntryActionCreated),
				ResourceType:      "ConfigMap",
				ResourceName:      "example",
				Data:              test.data,
				GithubActorClaims: test.claims,
			})
			if err != nil {
				t.Fatal(err)
			}
			generic := entry.(GenericKubernetesResourceActivityLogEntry)
			if (generic.GitHubActorClaims != nil) != test.wantClaims {
				t.Fatalf("unexpected claims: %+v", generic.GitHubActorClaims)
			}
			if test.wantClaims && generic.GitHubActorClaims.RunID != "123" {
				t.Fatalf("unexpected claims: %+v", generic.GitHubActorClaims)
			}
			if generic.Data.GitHubActorClaims != generic.GitHubActorClaims {
				t.Fatalf("deprecated data field does not match entry claims")
			}
			if generic.Data.Kind != "ConfigMap" {
				t.Fatalf("existing data was lost: %+v", generic.Data)
			}
		})
	}
}
