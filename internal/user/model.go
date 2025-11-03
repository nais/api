package user

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/user/usersql"
)

type (
	UserConnection = pagination.Connection[*User]
	UserEdge       = pagination.Edge[*User]
)

type User struct {
	UUID       uuid.UUID `json:"-"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	ExternalID string    `json:"externalId"`
	Admin      bool      `json:"admin"`
}

func (User) IsNode() {}

func (u *User) GetID() uuid.UUID       { return u.UUID }
func (u *User) Identity() string       { return u.Email }
func (u *User) IsServiceAccount() bool { return false }
func (u *User) IsAdmin() bool          { return u.Admin }
func (u *User) GCPTeamGroups(ctx context.Context) ([]string, error) {
	return ListGCPGroupsForUser(ctx, u.UUID)
}

func (u User) ID() ident.Ident {
	return NewIdent(u.UUID)
}

func toGraphUser(u *usersql.User) *User {
	return &User{
		UUID:       u.ID,
		Email:      u.Email,
		Name:       u.Name,
		ExternalID: u.ExternalID,
		Admin:      u.Admin,
	}
}

type UserOrder struct {
	Field     UserOrderField       `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

func (o *UserOrder) String() string {
	if o == nil {
		return ""
	}

	return strings.ToLower(o.Field.String() + ":" + o.Direction.String())
}

type UserOrderField string

const (
	UserOrderFieldName  UserOrderField = "NAME"
	UserOrderFieldEmail UserOrderField = "EMAIL"
)

func (e UserOrderField) IsValid() bool {
	switch e {
	case UserOrderFieldName, UserOrderFieldEmail:
		return true
	}
	return false
}

func (e UserOrderField) String() string {
	return string(e)
}

func (e *UserOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = UserOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid UserOrderField", str)
	}
	return nil
}

func (e UserOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type AuthenticatedUser interface {
	GetID() uuid.UUID
	Identity() string
	IsServiceAccount() bool
}
