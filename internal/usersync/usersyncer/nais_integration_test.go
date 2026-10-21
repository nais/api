//go:build integration_test

package usersyncer_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/test"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/usersync/usersyncer"
	"github.com/nais/api/internal/usersync/usersyncsql"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	zitadelclient "github.com/zitadel/zitadel-go/v3/pkg/client/user/v2"
	zitadeluser "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	admindirectoryv1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

type naisUserService struct {
	zitadeluser.UserServiceClient
	list func(*zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error)
}

type directoryTransport func(*http.Request) *http.Response

func (f directoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func (s naisUserService) ListUsers(_ context.Context, req *zitadeluser.ListUsersRequest, _ ...grpc.CallOption) (*zitadeluser.ListUsersResponse, error) {
	return s.list(req)
}

func naisUser(id, email, name string) *zitadeluser.User {
	return &zitadeluser.User{
		UserId: id,
		State:  zitadeluser.UserState_USER_STATE_ACTIVE,
		Type: &zitadeluser.User_Human{Human: &zitadeluser.HumanUser{
			Profile: &zitadeluser.HumanProfile{DisplayName: &name},
			Email:   &zitadeluser.HumanEmail{Email: email},
		}},
	}
}

func TestSyncNaisUsers(t *testing.T) {
	ctx := t.Context()
	log, _ := logrustest.NewNullLogger()
	container, dsn, err := startPostgresql(ctx, t, log)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("lifecycle preserves tenant users and grants only viewer access", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		ctx := database.NewLoaderContext(t.Context(), pool)
		q := usersyncsql.New(pool)
		outdated, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Old name", Email: "old@nais.io", ExternalID: "nais-1"})
		if err != nil {
			t.Fatal(err)
		}
		deleted, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Deleted", Email: "deleted@nais.io", ExternalID: "deleted"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO sessions (user_id, expires) VALUES ($1, NOW() + INTERVAL '1 hour')", deleted.ID); err != nil {
			t.Fatal(err)
		}

		snapshot := []*zitadeluser.User{
			naisUser("nais-1", "UPDATED@nais.io", "Updated name"),
			naisUser("nais-2", "new@nais.io", "New user"),
		}
		var reads int
		zc := &usersyncer.ZitadelWrapper{
			Client: &zitadelclient.Client{UserServiceClient: naisUserService{list: func(req *zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error) {
				reads++
				if req.Queries[1].GetOrganizationIdQuery().GetOrganizationId() != "nais-org" {
					t.Fatalf("unexpected organization: %v", req.Queries)
				}
				return &zitadeluser.ListUsersResponse{Result: snapshot}, nil
			}}},
			NaisOrganizationID: "nais-org",
		}
		// The same tenant snapshot is returned on every sync; its admin role and
		// normal default role must be unchanged by the additional user source.
		httpClient := &http.Client{Transport: directoryTransport(func(req *http.Request) *http.Response {
			if req.URL.Path == "/admin/directory/v1/users" {
				return test.Response("200 OK", `{"users":[{"id":"tenant-1","primaryEmail":"tenant@example.com","name":{"fullName":"Tenant user"}}]}`)
			}
			return test.Response("200 OK", `{"members":[{"id":"tenant-1","email":"tenant@example.com","type":"USER","status":"ACTIVE"}]}`)
		})}
		svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			t.Fatal(err)
		}
		syncer := usersyncer.New(pool, adminGroupPrefix, domain, zc, svc, log)
		for range 2 {
			if err := syncer.Sync(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if reads != 2 {
			t.Fatalf("expected one nais.io snapshot per sync, got %d", reads)
		}
		users, err := q.List(ctx)
		if err != nil || len(users) != 3 {
			t.Fatalf("expected three users, got %v, %v", users, err)
		}
		var tenant *usersyncsql.User
		for _, u := range users {
			if u.ExternalID == "tenant-1" {
				tenant = u
				continue
			}
			if u.Admin {
				t.Fatalf("nais.io user unexpectedly has admin access: %v", u)
			}
			if u.ExternalID == "nais-1" && (u.ID != outdated.ID || u.Name != "Updated name" || u.Email != "updated@nais.io") {
				t.Fatalf("existing nais.io identity was not updated in place: %v", u)
			}
			viewerCtx := user.NewLoaderContext(ctx, pool)
			viewerCtx = authz.NewLoaderContext(viewerCtx, pool)
			viewerCtx = authz.ContextWithActor(viewerCtx, &user.User{UUID: u.ID, Email: u.Email}, nil)
			if err := authz.CanCreateTeam(viewerCtx); err == nil {
				t.Fatal("nais.io viewer must not be able to create teams")
			}
			if err := authz.RequireGlobalAdmin(viewerCtx); err == nil {
				t.Fatal("nais.io viewer must not be a global admin")
			}
		}
		if tenant == nil || !tenant.Admin {
			t.Fatalf("tenant admin was changed: %v", tenant)
		}
		roles, err := q.ListRoles(ctx)
		if err != nil || len(roles) != 1 || roles[0].UserID != tenant.ID || roles[0].RoleName != "Team creator" {
			t.Fatalf("only the tenant should receive Team creator; got %v, %v", roles, err)
		}
		var sessions int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM sessions WHERE user_id = $1", deleted.ID).Scan(&sessions); err != nil || sessions != 0 {
			t.Fatalf("deleted user's sessions remain: %d, %v", sessions, err)
		}

		// One user disappears; the other becomes inactive. Neither can remain
		// in the API, even if they had an active session before this sync.
		if _, err := pool.Exec(ctx, "INSERT INTO sessions (user_id, expires) VALUES ($1, NOW() + INTERVAL '1 hour')", outdated.ID); err != nil {
			t.Fatal(err)
		}
		snapshot = []*zitadeluser.User{naisUser("nais-2", "new@nais.io", "New user")}
		snapshot[0].State = zitadeluser.UserState_USER_STATE_INACTIVE
		if err := syncer.Sync(ctx); err != nil {
			t.Fatal(err)
		}
		users, err = q.List(ctx)
		if err != nil || len(users) != 1 || users[0].ExternalID != "tenant-1" || !users[0].Admin {
			t.Fatalf("expected only the unchanged tenant admin, got %v, %v", users, err)
		}
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil || sessions != 0 {
			t.Fatalf("inactive/deleted users' sessions remain: %d, %v", sessions, err)
		}
	})

	t.Run("failed second page does not change database", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		q := usersyncsql.New(pool)
		before, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Keep", Email: "keep@nais.io", ExternalID: "keep"})
		if err != nil {
			t.Fatal(err)
		}
		wantErr := errors.New("second page failed")
		zc := &usersyncer.ZitadelWrapper{
			Client: &zitadelclient.Client{UserServiceClient: naisUserService{list: func(req *zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error) {
				if req.Query.Offset > 0 {
					return nil, wantErr
				}
				page := make([]*zitadeluser.User, 250)
				for i := range page {
					page[i] = naisUser(fmt.Sprint(i), fmt.Sprintf("new%d@nais.io", i), "New")
				}
				return &zitadeluser.ListUsersResponse{Result: page}, nil
			}}},
			NaisOrganizationID: "nais-org",
		}
		svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(test.NewTestHttpClient(func(*http.Request) *http.Response {
			return test.Response("200 OK", `{"users":[]}`)
		})))
		if err != nil {
			t.Fatal(err)
		}
		if err := usersyncer.New(pool, adminGroupPrefix, domain, zc, svc, log).Sync(ctx); !errors.Is(err, wantErr) {
			t.Fatalf("expected source failure, got %v", err)
		}
		users, err := q.List(ctx)
		if err != nil || len(users) != 1 || users[0].ID != before.ID {
			t.Fatalf("failed snapshot changed the database: %v, %v", users, err)
		}
	})

	t.Run("same email does not transfer another identity's privileges", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		q := usersyncsql.New(pool)
		before, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Existing", Email: "existing@nais.io", ExternalID: "other-identity"})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.AssignGlobalAdmin(ctx, before.ID); err != nil {
			t.Fatal(err)
		}
		zc := &usersyncer.ZitadelWrapper{
			Client: &zitadelclient.Client{UserServiceClient: naisUserService{list: func(*zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error) {
				return &zitadeluser.ListUsersResponse{Result: []*zitadeluser.User{naisUser("new-identity", before.Email, "New")}}, nil
			}}},
			NaisOrganizationID: "nais-org",
		}
		svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(test.NewTestHttpClient(func(*http.Request) *http.Response {
			return test.Response("200 OK", `{"users":[]}`)
		})))
		if err != nil {
			t.Fatal(err)
		}
		if err := usersyncer.New(pool, adminGroupPrefix, domain, zc, svc, log).Sync(ctx); err == nil {
			t.Fatal("expected conflicting email to abort sync, not adopt the existing identity")
		}
		users, err := q.List(ctx)
		if err != nil || len(users) != 1 || users[0].ID != before.ID || users[0].ExternalID != before.ExternalID || !users[0].Admin {
			t.Fatalf("conflicting identity was changed: %v, %v", users, err)
		}
	})

	for _, tc := range []struct {
		name       string
		remoteName string
		absent     bool
	}{
		{name: "unchanged nais.io display name", remoteName: "Nais viewer"},
		{name: "changed nais.io display name", remoteName: "Renamed nais.io viewer"},
		{name: "nais.io identity absent from current snapshot", absent: true},
	} {
		t.Run("Google email collision with "+tc.name, func(t *testing.T) {
			pool := getConnection(ctx, t, container, dsn, log)
			q := usersyncsql.New(pool)
			// Organization membership, not the email domain, determines the
			// source. This could also be a previously renamed nais.io account.
			before, err := q.Create(ctx, usersyncsql.CreateParams{
				Name: "Nais viewer", Email: "shared@example.com", ExternalID: "nais-subject",
			})
			if err != nil {
				t.Fatal(err)
			}
			var sessionID uuid.UUID
			if err := pool.QueryRow(ctx, "INSERT INTO sessions (user_id, expires) VALUES ($1, NOW() + INTERVAL '1 hour') RETURNING id", before.ID).Scan(&sessionID); err != nil {
				t.Fatal(err)
			}
			zc := &usersyncer.ZitadelWrapper{
				Client: &zitadelclient.Client{UserServiceClient: naisUserService{list: func(*zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error) {
					var users []*zitadeluser.User
					if !tc.absent {
						users = append(users, naisUser(before.ExternalID, before.Email, tc.remoteName))
					}
					return &zitadeluser.ListUsersResponse{Result: users}, nil
				}}},
				NaisOrganizationID: "nais-org",
			}
			httpClient := &http.Client{Transport: directoryTransport(func(req *http.Request) *http.Response {
				if req.URL.Path == "/admin/directory/v1/users" {
					return test.Response("200 OK", `{"users":[{"id":"tenant-admin","primaryEmail":"shared@example.com","name":{"fullName":"Tenant admin"}}]}`)
				}
				return test.Response("200 OK", `{"members":[{"id":"tenant-admin","email":"shared@example.com","type":"USER","status":"ACTIVE"}]}`)
			})}
			svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(httpClient))
			if err != nil {
				t.Fatal(err)
			}
			if err := usersyncer.New(pool, adminGroupPrefix, domain, zc, svc, log).Sync(ctx); err == nil {
				t.Error("expected conflicting Google identity to abort sync")
			}
			users, err := q.List(ctx)
			if err != nil || len(users) != 1 {
				t.Fatalf("unexpected users after collision: %v, %v", users, err)
			}
			got := users[0]
			if got.ID != before.ID || got.ExternalID != before.ExternalID || got.Name != before.Name || got.Email != before.Email || got.Admin {
				t.Errorf("Google identity adopted or changed the nais.io viewer: %+v", got)
			}
			var sessionUserID uuid.UUID
			var sessionIsAdmin bool
			if err := pool.QueryRow(ctx, "SELECT u.id, u.admin FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.id = $1", sessionID).Scan(&sessionUserID, &sessionIsAdmin); err != nil {
				t.Fatal(err)
			}
			if sessionUserID != before.ID || sessionIsAdmin {
				t.Errorf("existing nais.io session received tenant admin privileges: user=%s admin=%v", sessionUserID, sessionIsAdmin)
			}
		})
	}
}
