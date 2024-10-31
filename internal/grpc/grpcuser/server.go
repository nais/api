package grpcuser

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/grpc/grpcpagination"
	"github.com/nais/api/internal/grpc/grpcuser/grpcusersql"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	querier grpcusersql.Querier
	protoapi.UnimplementedUsersServer
}

func NewServer(querier grpcusersql.Querier) *Server {
	return &Server{
		querier: querier,
	}
}

func (u *Server) Get(ctx context.Context, r *protoapi.GetUserRequest) (*protoapi.GetUserResponse, error) {
	var user *grpcusersql.User
	var err error

	switch {
	case r.Id != "":
		var uid uuid.UUID
		uid, err = uuid.Parse(r.Id)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid ID, must be valid UUID")
		}
		user, err = u.querier.GetByID(ctx, uid)
	case r.Email != "":
		user, err = u.querier.GetByEmail(ctx, r.Email)
	case r.ExternalId != "":
		user, err = u.querier.GetByExternalID(ctx, r.ExternalId)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "must specify either ID, Email or External ID")
	}
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return &protoapi.GetUserResponse{
		User: toProtoUser(user),
	}, nil
}

func (u *Server) List(ctx context.Context, r *protoapi.ListUsersRequest) (*protoapi.ListUsersResponse, error) {
	limit, offset := grpcpagination.Pagination(r)
	users, err := u.querier.List(ctx, grpcusersql.ListParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list users")
	}

	total, err := u.querier.Count(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get users count")
	}

	resp := &protoapi.ListUsersResponse{
		PageInfo: grpcpagination.PageInfo(r, int(total)),
		Nodes:    make([]*protoapi.User, len(users)),
	}
	for i, user := range users {
		resp.Nodes[i] = toProtoUser(user)
	}

	return resp, nil
}

func toProtoUser(user *grpcusersql.User) *protoapi.User {
	return &protoapi.User{
		Id:         user.ID.String(),
		Name:       user.Name,
		Email:      user.Email,
		ExternalId: user.ExternalID,
	}
}
