package usersyncer

import (
	"context"
	"fmt"
	"strings"

	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/object/v2"
	zitadeluser "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	admindirectoryv1 "google.golang.org/api/admin/directory/v1"
)

// activeHumanUsers fetches a complete snapshot before the database is changed.
// A failed page must never be treated as a snapshot from which users can be deleted.
func (z *ZitadelWrapper) activeHumanUsers(ctx context.Context, organizationID string) ([]*sourceUser, error) {
	const limit = 250
	var users []*sourceUser
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
				return nil, fmt.Errorf("zitadel user is missing ID or email")
			}
			profile := u.GetHuman().GetProfile()
			users = append(users, &sourceUser{
				ID:    u.UserId,
				Email: strings.ToLower(u.GetHuman().GetEmail().GetEmail()),
				Name: admindirectoryv1.UserName{
					FullName:   profile.GetDisplayName(),
					GivenName:  profile.GetGivenName(),
					FamilyName: profile.GetFamilyName(),
				},
			})
		}
		if len(resp.Result) < limit {
			return users, nil
		}
	}
}
