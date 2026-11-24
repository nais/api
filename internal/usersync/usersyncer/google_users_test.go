package usersyncer

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/test"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	admindirectoryv1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

func TestGetGoogleUsers(t *testing.T) {
	const domain = "example.com"
	listUsers := func(req *http.Request) *http.Response {
		if req.URL.Path != "/admin/directory/v1/users" || req.URL.Query().Get("domain") != domain || req.URL.Query().Get("query") != "isSuspended=false" {
			t.Errorf("unexpected user listing URL: %s", req.URL)
		}
		return test.Response("200 OK", `{"users":[
			{"id":"1","primaryEmail":"one@example.com","name":{}},
			{"id":"2","primaryEmail":"two@example.com","name":{}},
			{"id":"3","primaryEmail":"three@example.com","name":{}},
			{"id":"4","primaryEmail":"four@example.com","name":{}},
			{"id":"5","primaryEmail":"five@example.com","name":{}}
		]}`)
	}
	listMembers := func(pageToken, status, body string) test.RoundTripFunc {
		return func(req *http.Request) *http.Response {
			q := req.URL.Query()
			if req.URL.Path != "/admin/directory/v1/groups/console-users@example.com/members" || q.Get("includeDerivedMembership") != "true" || q.Get("pageToken") != pageToken {
				t.Errorf("unexpected group membership URL: %s", req.URL)
			}
			return test.Response(status, body)
		}
	}

	for _, tc := range []struct {
		name      string
		prefix    string
		responses []test.RoundTripFunc
		wantIDs   []string
		wantCode  int
	}{
		{
			name:      "all eligible users by default",
			responses: []test.RoundTripFunc{listUsers},
			wantIDs:   []string{"1", "2", "3", "4", "5"},
		},
		{
			name:   "only active user members across pages",
			prefix: "console-users",
			responses: []test.RoundTripFunc{
				listMembers("", "200 OK", `{"nextPageToken":"next","members":[
					{"id":"1","type":"USER","status":"ACTIVE"},
					{"id":"2","type":"USER","status":"SUSPENDED"},
					{"id":"3","type":"GROUP","status":"ACTIVE"}
				]}`),
				listMembers("next", "200 OK", `{"members":[
					{"id":"4","type":"USER","status":"ACTIVE"},
					{"id":"external","type":"USER","status":"ACTIVE"}
				]}`),
				listUsers,
			},
			wantIDs: []string{"1", "4"},
		},
		{
			name:      "empty group admits no users",
			prefix:    "console-users",
			responses: []test.RoundTripFunc{listMembers("", "200 OK", `{"members":[]}`), listUsers},
		},
		{
			name:      "missing group aborts sync",
			prefix:    "console-users",
			responses: []test.RoundTripFunc{listMembers("", "404 Not Found", `{"error":{"code":404}}`)},
			wantCode:  http.StatusNotFound,
		},
		{
			name:   "incomplete group lookup aborts sync",
			prefix: "console-users",
			responses: []test.RoundTripFunc{
				listMembers("", "200 OK", `{"nextPageToken":"next","members":[{"id":"1","type":"USER","status":"ACTIVE"}]}`),
				listMembers("next", "403 Forbidden", `{"error":{"code":403}}`),
			},
			wantCode: http.StatusForbidden,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			log, _ := logrustest.NewNullLogger()
			svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(test.NewTestHttpClient(tc.responses...)))
			if err != nil {
				t.Fatal(err)
			}
			s := New(nil, "console-admins", tc.prefix, domain, nil, svc, log)
			if tc.wantCode != 0 {
				// A nil pool means Sync must return before attempting any writes.
				err := s.Sync(ctx)
				var apiErr *googleapi.Error
				if !errors.As(err, &apiErr) || apiErr.Code != tc.wantCode || !strings.Contains(err.Error(), "console-users@example.com") {
					t.Fatalf("expected group lookup error with status %d, got %v", tc.wantCode, err)
				}
				return
			}
			users, err := s.getGoogleUsers(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, user := range users {
				ids = append(ids, user.ID)
			}
			if diff := cmp.Diff(tc.wantIDs, ids); diff != "" {
				t.Errorf("synced user IDs (-want +got):\n%s", diff)
			}
		})
	}
}
