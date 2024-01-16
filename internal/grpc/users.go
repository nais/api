package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UsersServer struct {
	db database.UserRepo
	protoapi.UnimplementedUsersServer
}

func (u *UsersServer) Get(ctx context.Context, r *protoapi.GetUserRequest) (*protoapi.User, error) {
	var user *database.User
	var err error

	switch {
	case r.Id != "":
		var uid uuid.UUID
		uid, err = uuid.Parse(r.Id)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid ID, must be valid UUID")
		}
		user, err = u.db.GetUserByID(ctx, uid)
	case r.Email != "":
		user, err = u.db.GetUserByEmail(ctx, r.Email)
	case r.ExternalId != "":
		user, err = u.db.GetUserByExternalID(ctx, r.ExternalId)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "must specify either ID, Email or External ID")
	}
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return toProtoUser(user), nil
}

func (u *UsersServer) List(ctx context.Context, r *protoapi.ListUsersRequest) (*protoapi.ListUsersResponse, error) {
	limit, offset := pagination(r)
	users, total, err := u.db.GetUsers(ctx, offset, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list users: %s", err)
	}

	resp := &protoapi.ListUsersResponse{
		PageInfo: pageInfo(r, total),
	}
	for _, user := range users {
		resp.Nodes = append(resp.Nodes, toProtoUser(user))
	}

	return resp, nil
}

func toProtoUser(user *database.User) *protoapi.User {
	return &protoapi.User{
		Id:         user.ID.String(),
		Name:       user.Name,
		Email:      user.Email,
		ExternalId: user.ExternalID,
	}
}
