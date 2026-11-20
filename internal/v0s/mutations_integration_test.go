//go:build integration_test

package v0s_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/v0s/protoapi"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type writeClient struct {
	*client
	set    *protoapi.SetSuppressionsRequest
	delete *protoapi.DeleteSuppressionsRequest
}

func (c *writeClient) SetSuppressions(_ context.Context, req *protoapi.SetSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.SetSuppressionsResponse, error) {
	c.set = req
	return &protoapi.SetSuppressionsResponse{Suppressions: c.assessments}, nil
}

func (c *writeClient) DeleteSuppressions(_ context.Context, req *protoapi.DeleteSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.DeleteSuppressionsResponse, error) {
	c.delete = req
	return &protoapi.DeleteSuppressionsResponse{Deleted: c.assessments}, nil
}

func TestAuthorizedAssessmentBatchesRecordActivity(t *testing.T) {
	ctx := t.Context()
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logrustest.NewNullLogger()
	pool, err := database.NewPool(ctx, dsn, log, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `
		INSERT INTO teams (slug, purpose, slack_channel) VALUES ('team-one', 'test', '#test');
	`); err != nil {
		t.Fatal(err)
	}
	c := &writeClient{client: &client{}}
	ctx = authz.ContextWithActor(ctx, actor{}, []*authz.Role{{Name: "Team member", TargetTeamSlug: new(slug.Slug("team-one"))}})
	ctx = authz.NewLoaderContext(ctx, pool)
	ctx = activitylog.NewLoaderContext(ctx, pool)
	ctx = v0s.WithManager(ctx, &v0s.Manager{Vulnerabilities: c, Suppressions: c, SuppressionsEnabled: true})
	keys := []*v0s.VulnerabilitySuppressionKeyInput{}
	items := []*v0s.NewVulnerabilitySuppressionInput{}
	for _, name := range []string{"app", "job"} {
		key := &v0s.VulnerabilitySuppressionKeyInput{
			Environment: "dev", Workload: name, ImageName: "repo/shared",
			PackageKey: "pkg:npm/example", VulnerabilityIdentifier: "CVE-1",
		}
		keys = append(keys, key)
		items = append(items, &v0s.NewVulnerabilitySuppressionInput{
			Key: key, State: v0s.VulnerabilitySuppressionStateAcceptedRisk, Reason: "assessed",
		})
		c.assessments = append(c.assessments, &protoapi.Suppression{
			Team: "team-one", Environment: "dev", Workload: name, ImageName: "repo/shared",
			PackageKey: "pkg:npm/example", VulnerabilityId: "CVE-1",
			State:  protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK,
			Detail: "assessed", UpdatedBy: "actor-1", UpdatedAt: timestamppb.Now(),
		})
	}
	items[1].Reason = ""
	if _, err := v0s.SetSuppressions(ctx, v0s.SetVulnerabilitySuppressionsInput{Team: "team-one", Items: items}); err == nil || c.set != nil {
		t.Fatal("an invalid batch reached the backend")
	}
	items[1].Reason = "assessed"
	set, err := v0s.SetSuppressions(ctx, v0s.SetVulnerabilitySuppressionsInput{Team: "team-one", Items: items})
	if err != nil {
		t.Fatal(err)
	}
	if c.set.GetTeam() != "team-one" || c.set.GetActor() != "actor-1" || len(c.set.GetSuppressions()) != 2 ||
		c.set.GetSuppressions()[1].GetKey().GetWorkload() != "job" || len(set.Suppressions) != 2 {
		t.Fatalf("assessment batch or authenticated actor was not preserved: %v", c.set)
	}
	deleted, err := v0s.DeleteSuppressions(ctx, v0s.DeleteVulnerabilitySuppressionsInput{Team: "team-one", Keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	if c.delete.GetTeam() != "team-one" || len(c.delete.GetKeys()) != 2 || len(deleted.Deleted) != 2 {
		t.Fatalf("deletion batch was not preserved: %v", c.delete)
	}
	rows, err := pool.Query(ctx, `
		SELECT id, action, actor, team_slug, environment, resource_name, data
		FROM activity_log_entries
		WHERE resource_type = 'VULNERABILITY_SUPPRESSION'
		ORDER BY action, resource_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id uuid.UUID
		var action, actorID, team, environment, name string
		var data []byte
		if err := rows.Scan(&id, &action, &actorID, &team, &environment, &name, &data); err != nil {
			t.Fatal(err)
		}
		var entry v0s.VulnerabilitySuppressionActivityLogEntryData
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatal(err)
		}
		if actorID != "actor-1" || team != "team-one" || environment != "dev" ||
			(action != "UPDATED" && action != "DELETED") || entry.Assessment == nil ||
			entry.Assessment.Workload != name || entry.Assessment.Reason != "assessed" {
			t.Fatalf("incorrect activity: %s, %s, %s, %s, %s, %s", action, actorID, team, environment, name, data)
		}
		entries := set.Suppressions
		if action == "DELETED" {
			entries = deleted.Deleted
		}
		want := entries[0]
		if name == "job" {
			want = entries[1]
		}
		if diff := cmp.Diff(want, entry.Assessment); diff != "" {
			t.Fatalf("activity lost assessment details (-want +got):\n%s", diff)
		}
		loaded, err := activitylog.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		typed, ok := loaded.(v0s.VulnerabilitySuppressionActivityLogEntry)
		if !ok {
			t.Fatalf("assessment activity could not be read through its domain type: %T", loaded)
		}
		if diff := cmp.Diff(entry.Assessment, typed.Data.Assessment); diff != "" {
			t.Fatalf("reading activity lost assessment details (-want +got):\n%s", diff)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("recorded %d assessment activities, want two updates and two deletions", count)
	}
}
