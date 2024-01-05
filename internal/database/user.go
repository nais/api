package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/sqlc"
)

func (d *database) CreateUser(ctx context.Context, name, email, externalID string) (*User, error) {
	user, err := d.querier.CreateUser(ctx, sqlc.CreateUserParams{
		Name:       name,
		Email:      email,
		ExternalID: externalID,
	})
	if err != nil {
		return nil, err
	}

	return wrapUser(user), nil
}

func (d *database) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	return d.querier.DeleteUser(ctx, userID)
}

func (d *database) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user, err := d.querier.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return wrapUser(user), nil
}

func (d *database) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user, err := d.querier.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return wrapUser(user), nil
}

func (d *database) GetUserByExternalID(ctx context.Context, externalID string) (*User, error) {
	user, err := d.querier.GetUserByExternalID(ctx, externalID)
	if err != nil {
		return nil, err
	}

	return wrapUser(user), nil
}

func (d *database) UpdateUser(ctx context.Context, userID uuid.UUID, name, email, externalID string) (*User, error) {
	user, err := d.querier.UpdateUser(ctx, sqlc.UpdateUserParams{
		Email:      email,
		ExternalID: externalID,
		ID:         userID,
		Name:       name,
	})
	if err != nil {
		return nil, err
	}

	return wrapUser(user), nil
}

func (d *database) GetUsers(ctx context.Context, offset, limit int) ([]*User, int, error) {
	var users []*sqlc.User
	var err error
	users, err = d.querier.GetUsers(ctx, sqlc.GetUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := d.querier.GetUsersCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	return wrapUsers(users), int(total), nil
}

func (d *database) GetAllUsers(ctx context.Context) ([]*User, error) {
	var users []*sqlc.User
	var err error
	users, err = d.querier.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	return wrapUsers(users), nil
}

func wrapUsers(users []*sqlc.User) []*User {
	result := make([]*User, 0)
	for _, user := range users {
		result = append(result, wrapUser(user))
	}

	return result
}

func wrapUser(user *sqlc.User) *User {
	return &User{User: user}
}

func (d *database) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*Role, error) {
	userRoles, err := d.querier.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles := make([]*Role, 0, len(userRoles))
	for _, userRole := range userRoles {
		role, err := d.roleFromRoleBinding(ctx, userRole.RoleName, userRole.TargetServiceAccountID, userRole.TargetTeamSlug)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, nil
}
