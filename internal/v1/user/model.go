package user

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/user/usersql"
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
}

func (User) IsNode() {}

func (u User) ID() ident.Ident {
	return newIdent(u.UUID)
}

func toGraphUser(u *usersql.User) *User {
	return &User{
		UUID:       u.ID,
		Email:      u.Email,
		Name:       u.Name,
		ExternalID: u.ExternalID,
	}
}

type UserOrder struct {
	Field     UserOrderField         `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
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