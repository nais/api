package usersyncer

import (
	"context"
	"errors"
	"fmt"
	"testing"

	zitadelclient "github.com/zitadel/zitadel-go/v3/pkg/client/user/v2"
	zitadeluser "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	"google.golang.org/grpc"
)

type userService struct {
	zitadeluser.UserServiceClient
	list func(context.Context, *zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error)
}

func (s userService) ListUsers(ctx context.Context, req *zitadeluser.ListUsersRequest, _ ...grpc.CallOption) (*zitadeluser.ListUsersResponse, error) {
	return s.list(ctx, req)
}

func activeUser(id, email string) *zitadeluser.User {
	return &zitadeluser.User{
		UserId: id,
		State:  zitadeluser.UserState_USER_STATE_ACTIVE,
		Type: &zitadeluser.User_Human{Human: &zitadeluser.HumanUser{
			Email: &zitadeluser.HumanEmail{Email: email},
		}},
	}
}

func TestActiveHumanUsers(t *testing.T) {
	var offsets []uint64
	z := &ZitadelWrapper{Client: &zitadelclient.Client{UserServiceClient: userService{
		list: func(ctx context.Context, req *zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error) {
			offsets = append(offsets, req.Query.Offset)
			if req.Query.Limit != 250 {
				t.Fatalf("unexpected page size: %d", req.Query.Limit)
			}
			if len(req.Queries) != 3 || req.Queries[0].GetTypeQuery().GetType() != zitadeluser.Type_TYPE_HUMAN ||
				req.Queries[1].GetOrganizationIdQuery().GetOrganizationId() != "nais-org" ||
				req.Queries[2].GetStateQuery().GetState() != zitadeluser.UserState_USER_STATE_ACTIVE {
				t.Fatalf("unexpected user filters: %v", req.Queries)
			}
			if req.Query.Offset == 0 {
				users := make([]*zitadeluser.User, 250)
				for i := range users {
					users[i] = activeUser(fmt.Sprint(i), fmt.Sprintf("user%d@nais.io", i))
				}
				return &zitadeluser.ListUsersResponse{Result: users}, nil
			}
			inactive := activeUser("inactive", "inactive@nais.io")
			inactive.State = zitadeluser.UserState_USER_STATE_INACTIVE
			last := activeUser("last", "LAST@nais.io")
			last.GetHuman().Profile = &zitadeluser.HumanProfile{DisplayName: new("Last user")}
			return &zitadeluser.ListUsersResponse{Result: []*zitadeluser.User{
				last, inactive,
				{UserId: "machine", State: zitadeluser.UserState_USER_STATE_ACTIVE, Type: &zitadeluser.User_Machine{Machine: &zitadeluser.MachineUser{}}},
			}}, nil
		},
	}}}
	users, err := z.activeHumanUsers(t.Context(), "nais-org")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 251 || len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 250 {
		t.Fatalf("got %d users, offsets %v", len(users), offsets)
	}
	last := users[len(users)-1]
	if last.ID != "last" || last.Email != "last@nais.io" || last.Name.FullName != "Last user" || last.isTenantUser {
		t.Fatalf("unexpected mapped nais.io user: %+v", last)
	}
}

func TestActiveHumanUsersDiscardsPartialSnapshot(t *testing.T) {
	wantErr := errors.New("Zitadel unavailable")
	z := &ZitadelWrapper{Client: &zitadelclient.Client{UserServiceClient: userService{
		list: func(ctx context.Context, req *zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error) {
			if req.Query.Offset != 0 {
				return nil, wantErr
			}
			users := make([]*zitadeluser.User, 250)
			for i := range users {
				users[i] = activeUser(fmt.Sprint(i), "user@nais.io")
			}
			return &zitadeluser.ListUsersResponse{Result: users}, nil
		},
	}}}
	users, err := z.activeHumanUsers(t.Context(), "nais-org")
	if !errors.Is(err, wantErr) || users != nil {
		t.Fatalf("expected no partial snapshot and %v, got %v, %v", wantErr, users, err)
	}
}

func TestActiveHumanUsersRejectsIncompleteIdentity(t *testing.T) {
	for _, u := range []*zitadeluser.User{activeUser("", "user@nais.io"), activeUser("id", "")} {
		z := &ZitadelWrapper{Client: &zitadelclient.Client{UserServiceClient: userService{
			list: func(context.Context, *zitadeluser.ListUsersRequest) (*zitadeluser.ListUsersResponse, error) {
				return &zitadeluser.ListUsersResponse{Result: []*zitadeluser.User{u}}, nil
			},
		}}}
		if _, err := z.activeHumanUsers(t.Context(), "nais-org"); err == nil {
			t.Fatal("expected incomplete identity to abort snapshot")
		}
	}
}
