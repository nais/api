package grpcserviceaccount

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/grpc/grpcserviceaccount/grpcserviceaccountsql"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	querier grpcserviceaccountsql.Querier
	protoapi.UnimplementedServiceAccountsServer
}

func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{
		querier: grpcserviceaccountsql.New(pool),
	}
}

func (s *Server) Create(ctx context.Context, req *protoapi.CreateServiceAccountRequest) (*protoapi.CreateServiceAccountResponse, error) {
	teamSlug := slug.Slug(req.GetTeamSlug())
	if teamSlug == "" {
		return nil, status.Errorf(codes.InvalidArgument, "team_slug is required")
	}
	if req.GetName() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	team := teamSlug
	row, err := s.querier.Create(ctx, grpcserviceaccountsql.CreateParams{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		TeamSlug:    &team,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create service account: %s", err)
	}

	if role := req.GetRoleName(); role != "" {
		err := s.querier.AssignRole(ctx, grpcserviceaccountsql.AssignRoleParams{
			ServiceAccountID: row.ID,
			RoleName:         role,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to assign role %q: %s", role, err)
		}
	}

	return protoapi.CreateServiceAccountResponse_builder{
		Id:      row.ID.String(),
		Created: row.Created,
	}.Build(), nil
}

func (s *Server) CreateToken(ctx context.Context, req *protoapi.CreateServiceAccountTokenRequest) (*protoapi.CreateServiceAccountTokenResponse, error) {
	serviceAccountID, err := uuid.Parse(req.GetServiceAccountId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "service_account_id must be a valid UUID: %s", err)
	}
	if req.GetName() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	secret, err := serviceaccount.GenerateToken()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token")
	}
	hashed, err := serviceaccount.HashToken(secret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to hash token")
	}

	// expires_at is day-granular on the server side.
	expiresAt := pgtype.Date{}
	if req.HasExpiresAt() && req.GetExpiresAt().IsValid() {
		expiresAt = pgtype.Date{Time: req.GetExpiresAt().AsTime(), Valid: true}
	}

	id, err := s.querier.CreateToken(ctx, grpcserviceaccountsql.CreateTokenParams{
		ExpiresAt:        expiresAt,
		Name:             req.GetName(),
		Description:      req.GetDescription(),
		Token:            hashed,
		ServiceAccountID: serviceAccountID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create token: %s", err)
	}

	return protoapi.CreateServiceAccountTokenResponse_builder{
		Id:     id.String(),
		Secret: secret,
	}.Build(), nil
}
