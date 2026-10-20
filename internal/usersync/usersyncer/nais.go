package usersyncer

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/api/internal/usersync/usersyncsql"
	"github.com/sirupsen/logrus"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/object/v2"
	zitadeluser "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
)

// activeHumanUsers fetches a complete snapshot before the database is changed.
// A failed page must never be treated as a snapshot from which users can be deleted.
func (z *ZitadelWrapper) activeHumanUsers(ctx context.Context, organizationID string) ([]*zitadeluser.User, error) {
	const limit = 250
	var users []*zitadeluser.User
	for offset := uint64(0); ; offset += limit {
		resp, err := z.ListUsers(ctx, &zitadeluser.ListUsersRequest{
			Query: &object.ListQuery{Offset: offset, Limit: limit},
			Queries: []*zitadeluser.SearchQuery{
				{Query: &zitadeluser.SearchQuery_TypeQuery{TypeQuery: &zitadeluser.TypeQuery{Type: zitadeluser.Type_TYPE_HUMAN}}},
				{Query: &zitadeluser.SearchQuery_OrganizationIdQuery{OrganizationIdQuery: &zitadeluser.OrganizationIdQuery{OrganizationId: organizationID}}},
				{Query: &zitadeluser.SearchQuery_StateQuery{StateQuery: &zitadeluser.StateQuery{State: zitadeluser.UserState_USER_STATE_ACTIVE}}},
			},
			SortingColumn: zitadeluser.UserFieldName_USER_FIELD_NAME_EMAIL,
		})
		if err != nil {
			return nil, err
		}
		for _, u := range resp.Result {
			if u.GetHuman() == nil || u.GetState() != zitadeluser.UserState_USER_STATE_ACTIVE {
				continue
			}
			if u.GetUserId() == "" || u.GetHuman().GetEmail().GetEmail() == "" {
				return nil, fmt.Errorf("Zitadel user is missing ID or email")
			}
			users = append(users, u)
		}
		if len(resp.Result) < limit {
			return users, nil
		}
	}
}

// syncNaisUsers imports existing Zitadel identities without tenant default roles
// (notably Team creator), admin roles or team membership. Reading is available to
// authenticated users without assigning an additional role.
func syncNaisUsers(ctx context.Context, querier *usersyncsql.Queries, incoming []*zitadeluser.User, existing *userMap, log logrus.FieldLogger) error {
	for _, remote := range incoming {
		name := remote.GetHuman().GetProfile().GetDisplayName()
		email := strings.ToLower(remote.GetHuman().GetEmail().GetEmail())
		local, exists := existing.byExternalID[remote.UserId]
		if !exists {
			// Do not adopt a different identity's roles or sessions by matching on email.
			created, err := querier.Create(ctx, usersyncsql.CreateParams{
				Name: name, Email: email, ExternalID: remote.UserId,
			})
			if err != nil {
				return fmt.Errorf("create user %q: %w", email, err)
			}
			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action: usersyncsql.UsersyncLogEntryActionCreateUser,
				UserID: created.ID, UserName: name, UserEmail: email,
			}); err != nil {
				log.WithError(err).Error("create user sync log entry")
			}
			continue
		}

		if local.Name != name || local.Email != email {
			if err := querier.Update(ctx, usersyncsql.UpdateParams{
				ID: local.ID, Name: name, Email: email, ExternalID: remote.UserId,
			}); err != nil {
				return fmt.Errorf("update user %q: %w", email, err)
			}
			if err := querier.CreateLogEntry(ctx, usersyncsql.CreateLogEntryParams{
				Action: usersyncsql.UsersyncLogEntryActionUpdateUser,
				UserID: local.ID, UserName: name, UserEmail: email,
				OldUserName: &local.Name, OldUserEmail: &local.Email,
			}); err != nil {
				log.WithError(err).Error("create user sync log entry")
			}
		}
		delete(existing.byID, local.ID)
	}
	return nil
}
