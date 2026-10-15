package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/slug"
	liberatorv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"github.com/nais/pgrator/pkg/api"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic/fake"
)

func TestCreatePostgresUserLabels(t *testing.T) {
	for _, tt := range []struct {
		name   string
		labels []*model.ResourceLabel
		want   []*model.ResourceLabel
	}{
		{name: "without labels", want: []*model.ResourceLabel{}},
		{
			name:   "with labels",
			labels: []*model.ResourceLabel{{Key: "tier", Value: "backend"}},
			want:   []*model.ResourceLabel{{Key: "tier", Value: "backend"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pg := &nais_io_v1.Postgres{ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"}}
			kubernetes.SetManagedByConsoleLabel(pg)
			setPostgresUserLabels(pg, tt.labels)

			got := model.UserLabels(pg.GetLabels())
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("user labels mismatch (-want +got):\n%s", diff)
			}
			if got := pg.GetLabels()["app.kubernetes.io/managed-by"]; got != "console" {
				t.Errorf("managed-by label = %q, want console", got)
			}

			object, err := toUnstructuredWithoutZeroResources(pg)
			if err != nil {
				t.Fatal(err)
			}
			created, err := toPostgres(object, "dev")
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, created.Labels); diff != "" {
				t.Errorf("returned labels mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdatePostgresUserLabels(t *testing.T) {
	systemLabels := map[string]string{
		"app":                          "orders",
		"app.kubernetes.io/managed-by": "console",
		"nais.io/platform":             "managed",
		"team":                         "team-a",
	}
	for _, tt := range []struct {
		name       string
		labels     []*model.ResourceLabel
		wantLabels []*model.ResourceLabel
		wantChange bool
	}{
		{
			name:       "omitted labels preserve existing labels",
			wantLabels: []*model.ResourceLabel{{Key: "owner", Value: "payments"}, {Key: "tier", Value: "backend"}},
		},
		{
			name:       "labels replace existing user labels",
			labels:     []*model.ResourceLabel{{Key: "owner", Value: "ledger"}},
			wantLabels: []*model.ResourceLabel{{Key: "owner", Value: "ledger"}},
			wantChange: true,
		},
		{
			name:       "empty labels remove user labels",
			labels:     []*model.ResourceLabel{},
			wantLabels: []*model.ResourceLabel{},
			wantChange: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			existingLabels := map[string]string{
				"app":                          systemLabels["app"],
				"app.kubernetes.io/managed-by": systemLabels["app.kubernetes.io/managed-by"],
				"nais.io/platform":             systemLabels["nais.io/platform"],
				"team":                         systemLabels["team"],
				"owner":                        "payments",
				"tier":                         "backend",
			}
			pg := &nais_io_v1.Postgres{ObjectMeta: metav1.ObjectMeta{Labels: existingLabels}}
			changes, err := updatePostgresUserLabels(pg, UpdatePostgresInput{Labels: tt.labels})
			if err != nil {
				t.Fatal(err)
			}
			if (len(changes) > 0) != tt.wantChange {
				t.Fatalf("changes = %v, want change %t", changes, tt.wantChange)
			}
			if diff := cmp.Diff(tt.wantLabels, model.UserLabels(pg.GetLabels())); diff != "" {
				t.Errorf("user labels mismatch (-want +got):\n%s", diff)
			}
			for key, want := range systemLabels {
				if got := pg.GetLabels()[key]; got != want {
					t.Errorf("system label %q = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestPostgresLabelValidation(t *testing.T) {
	validCreate := CreatePostgresInput{
		Name: "orders", EnvironmentName: "dev", TeamSlug: "team-a", MajorVersion: "18",
	}
	validUpdate := UpdatePostgresInput{Name: "orders", EnvironmentName: "dev", TeamSlug: "team-a"}
	for _, tt := range []struct {
		name   string
		labels []*model.ResourceLabel
	}{
		{name: "reserved key", labels: []*model.ResourceLabel{{Key: "app", Value: "orders"}}},
		{name: "platform key", labels: []*model.ResourceLabel{{Key: "nais.io/owner", Value: "team-a"}}},
		{name: "invalid key", labels: []*model.ResourceLabel{{Key: "invalid/key/", Value: "value"}}},
		{name: "invalid value", labels: []*model.ResourceLabel{{Key: "tier", Value: "backend/value"}}},
		{name: "duplicate keys", labels: []*model.ResourceLabel{{Key: "tier", Value: "backend"}, {Key: "tier", Value: "frontend"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			create := validCreate
			create.Labels = tt.labels
			if err := create.Validate(context.Background()); err == nil {
				t.Fatal("CreatePostgresInput.Validate() expected an error")
			}
			update := validUpdate
			update.Labels = tt.labels
			if err := update.Validate(context.Background()); err == nil {
				t.Fatal("UpdatePostgresInput.Validate() expected an error")
			}
		})
	}
}

func TestUpdatePostgresWithoutLabelsKeepsExistingLabelsAndUpdatesOtherFields(t *testing.T) {
	pg := &nais_io_v1.Postgres{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"owner": "payments", "app": "orders"}},
		Spec:       nais_io_v1.PostgresSpec{MajorVersion: "18"},
	}
	input := UpdatePostgresInput{HighAvailability: new(true), CPU: new("200m")}

	labelChanges, err := updatePostgresUserLabels(pg, input)
	if err != nil {
		t.Fatal(err)
	}
	haChanges, err := updateHighAvailability(pg, input)
	if err != nil {
		t.Fatal(err)
	}
	resourceChanges, err := updateResources(pg, input)
	if err != nil {
		t.Fatal(err)
	}

	if len(labelChanges) != 0 || len(haChanges) != 1 || len(resourceChanges) != 1 {
		t.Fatalf("update changes = labels %v, high availability %v, resources %v", labelChanges, haChanges, resourceChanges)
	}
	if got := pg.GetLabels(); got["owner"] != "payments" || got["app"] != "orders" {
		t.Errorf("labels changed without labels input: %v", got)
	}
	if !pg.Spec.HighAvailability || pg.Spec.Resources.Cpu.String() != "200m" {
		t.Errorf("non-label fields were not updated: %+v", pg.Spec)
	}
}

func TestUniquePostgresUses(t *testing.T) {
	tests := []struct {
		name    string
		uses    []liberatorv1.PostgresUse
		want    []liberatorv1.PostgresUse
		wantErr string
	}{
		{
			name: "duplicate references to the same branch collapse",
			uses: []liberatorv1.PostgresUse{{Name: "orders", Branch: "main"}, {Name: "orders", Branch: "main"}},
			want: []liberatorv1.PostgresUse{{Name: "orders", Branch: "main"}},
		},
		{
			name:    "one Postgres cannot reference multiple branches",
			uses:    []liberatorv1.PostgresUse{{Name: "orders", Branch: "main"}, {Name: "orders", Branch: "green"}},
			wantErr: "multiple branches for Postgres \"orders\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := uniquePostgresUses(tt.uses)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("uniquePostgresUses() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("uniquePostgresUses() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("uniquePostgresUses() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewPostgresAccessResource(t *testing.T) {
	expiresAt := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	resource := newPostgresAccessResource(CreatePostgresAccessInput{
		Postgres: "orders", Branch: "main",
		TeamSlug:        slug.Slug("team-a"),
		EnvironmentName: "dev",
		AccessLevel:     PostgresAccessLevelReadWrite,
	}, "user@example.com", "postgres-access-12345678", expiresAt)

	if got, want := resource.GetAPIVersion(), "nais.io/v1"; got != want {
		t.Errorf("apiVersion = %q, want %q", got, want)
	}
	if got, want := resource.GetKind(), "PostgresAccess"; got != want {
		t.Errorf("kind = %q, want %q", got, want)
	}
	if got, want := resource.GetName(), "postgres-access-12345678"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := resource.GetNamespace(), "team-a"; got != want {
		t.Errorf("namespace = %q, want %q", got, want)
	}

	spec, found, err := unstructured.NestedMap(resource.Object, "spec")
	if err != nil || !found {
		t.Fatalf("spec = (%v, %t, %v), want a spec", spec, found, err)
	}
	wantSpec := map[string]any{
		"postgresBranch": nais_io_v1.PostgresBranchObjectName("orders", "main"),
		"username":       "user@example.com",
		"accessLevel":    "readwrite",
		"expiresAt":      "2026-09-17T12:00:00Z",
	}
	if diff := cmp.Diff(wantSpec, spec); diff != "" {
		t.Errorf("spec mismatch (-want +got):\n%s", diff)
	}
}

func TestWaitForPostgresAccessDeletion(t *testing.T) {
	access := newPostgresAccessResource(CreatePostgresAccessInput{TeamSlug: slug.Slug("team-a")}, "user@example.com", "postgres-access-test", time.Now())
	access.SetUID(types.UID("original-uid"))
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), access)
	accesses := client.Resource(postgresAccessGVR()).Namespace(access.GetNamespace())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := waitForPostgresAccessDeletion(ctx, accesses, access.GetName(), access.GetUID()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("still-present access: got %v, want deadline exceeded", err)
	}
	if err := accesses.Delete(context.Background(), access.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := waitForPostgresAccessDeletion(context.Background(), accesses, access.GetName(), access.GetUID()); err != nil {
		t.Fatalf("deleted access: %v", err)
	}
	replacement := access.DeepCopy()
	replacement.SetUID(types.UID("replacement-uid"))
	if _, err := accesses.Create(context.Background(), replacement, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := waitForPostgresAccessDeletion(context.Background(), accesses, access.GetName(), access.GetUID()); err != nil {
		t.Fatalf("replacement must not be mistaken for the old access: %v", err)
	}
}

func TestCreatePostgresAccessTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     string
		want    time.Duration
		wantErr string
	}{
		{name: "default", want: time.Hour},
		{name: "requested", ttl: "30m", want: 30 * time.Minute},
		{name: "maximum", ttl: "1h", want: time.Hour},
		{name: "invalid", ttl: "tomorrow", wantErr: "TTL must be a Go duration"},
		{name: "zero", ttl: "0s", wantErr: "TTL must be positive"},
		{name: "too long", ttl: "1h1m", wantErr: "TTL cannot exceed 1h0m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (CreatePostgresAccessInput{TTL: tt.ttl}).accessTTL()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("accessTTL() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("accessTTL() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("accessTTL() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPostgresAccessState(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	tests := []struct {
		name      string
		expiresAt time.Time
		status    *nais_io_v1.PostgresAccessStatus
		wantState PostgresAccessState
		wantMsg   string
	}{
		{
			name:      "expired",
			expiresAt: past,
			wantState: PostgresAccessStateExpired,
			wantMsg:   "access has expired",
		},
		{
			name:      "pending without status",
			expiresAt: future,
			wantState: PostgresAccessStatePending,
			wantMsg:   "waiting for controller",
		},
		{
			name:      "ready",
			expiresAt: future,
			status:    &nais_io_v1.PostgresAccessStatus{BaseStatus: api.BaseStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, Message: "database role and relay mapping are ready"}}}},
			wantState: PostgresAccessStateReady,
			wantMsg:   "database role and relay mapping are ready",
		},
		{
			name:      "failed unsupported access level",
			expiresAt: future,
			status:    &nais_io_v1.PostgresAccessStatus{BaseStatus: api.BaseStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse, Reason: "UnsupportedAccessLevel", Message: "readwritecreate requires an instance initialized with the app_readwritecreate group role"}}}},
			wantState: PostgresAccessStateFailed,
			wantMsg:   "readwritecreate requires an instance initialized with the app_readwritecreate group role",
		},
		{
			name:      "pending waiting on relay",
			expiresAt: future,
			status:    &nais_io_v1.PostgresAccessStatus{BaseStatus: api.BaseStatus{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse, Message: "waiting for relay"}}}},
			wantState: PostgresAccessStatePending,
			wantMsg:   "waiting for relay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotMsg := postgresAccessState(&nais_io_v1.PostgresAccess{Status: tt.status}, tt.expiresAt)
			if gotState != tt.wantState {
				t.Errorf("state = %q, want %q", gotState, tt.wantState)
			}
			if gotMsg != tt.wantMsg {
				t.Errorf("message = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestPostgresAccessConnectionDetails(t *testing.T) {
	now := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	ready := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{"name": "access"},
			"spec":     map[string]any{"expiresAt": "2026-09-17T13:00:00Z"},
			"status": map[string]any{
				"databaseRole":   "personal-role",
				"relayAccess":    "access",
				"relayEndpoint":  "https://relay.external.dev.nav.cloud.nais.io:8443",
				"tokenSecret":    "access-relay-token",
				"serverName":     "pg-orders-rw.team.svc.cluster.local",
				"serverCASecret": "pg-orders-ca",
				"conditions":     []any{map[string]any{"type": "Ready", "status": "True"}},
			},
		}}
	}

	tests := []struct {
		name string
		edit func(*unstructured.Unstructured)
		want string
	}{
		{name: "ready"},
		{name: "expired", edit: func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, "2026-09-17T12:00:00Z", "spec", "expiresAt")
		}, want: "expired"},
		{name: "not ready", edit: func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, []any{map[string]any{"type": "Ready", "status": "False"}}, "status", "conditions")
		}, want: "not ready"},
		{name: "missing token secret name", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "tokenSecret")
		}, want: "not ready"},
		{name: "missing relay mapping", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "relayAccess")
		}, want: "not ready"},
		{name: "missing relay endpoint", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "relayEndpoint")
		}, want: "not ready"},
		{name: "missing server name", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "serverName")
		}, want: "not ready"},
		{name: "missing server CA reference", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "serverCASecret")
		}, want: "not ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := ready()
			if tt.edit != nil {
				tt.edit(u)
			}
			got, secretName, err := postgresAccessConnectionDetails(u, now)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("postgresAccessConnectionDetails: %v", err)
			}
			if secretName != "access-relay-token" {
				t.Errorf("secret name = %q", secretName)
			}
			if got == nil || got.RelayEndpoint != "https://relay.external.dev.nav.cloud.nais.io:8443" {
				t.Fatalf("connection endpoint = %v, want status endpoint", got)
			}
		})
	}
}

func TestPostgresAccessNameIsStablePerUserAndBranch(t *testing.T) {
	name := postgresAccessName("user@example.com", "orders-main-abc")
	if name != postgresAccessName("user@example.com", "orders-main-abc") {
		t.Error("name must be stable, so a second request for the same pair conflicts")
	}
	for _, other := range []string{
		postgresAccessName("other@example.com", "orders-main-abc"),
		postgresAccessName("user@example.com", "orders-recovered-abc"),
		// Shifting the boundary between user and branch must not collide.
		postgresAccessName("user@example.comorders", "-main-abc"),
	} {
		if other == name {
			t.Errorf("different user/branch produced the same name %q", name)
		}
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		t.Errorf("name %q is not a valid resource name: %v", name, errs)
	}
}

func TestOmittedResourcesAreNotSent(t *testing.T) {
	cpu := "500m"
	pg := &nais_io_v1.Postgres{}
	if err := applyResources(&pg.Spec.Resources, &cpu, nil, nil); err != nil {
		t.Fatal(err)
	}

	obj, err := toUnstructuredWithoutZeroResources(pg)
	if err != nil {
		t.Fatal(err)
	}

	resources, _, _ := unstructured.NestedMap(obj.Object, "spec", "resources")
	if got := resources["cpu"]; got != "500m" {
		t.Errorf("cpu = %v, want 500m", got)
	}
	for _, field := range []string{"memory", "diskSize"} {
		if v, found := resources[field]; found {
			t.Errorf("%s = %v was sent, but must be left out so the CRD default applies", field, v)
		}
	}

	empty, err := toUnstructuredWithoutZeroResources(&nais_io_v1.Postgres{})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, _ := unstructured.NestedMap(empty.Object, "spec", "resources"); found {
		t.Error("spec.resources must be left out entirely when nothing is set")
	}
}
