package user

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/user/usersql"
)

type (
	UserConnection = pagination.Connection[*User]
	UserEdge       = pagination.Edge[*User]
)

type User struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	ExternalID string    `json:"externalId"`
	IsAdmin    bool      `json:"isAdmin"`
}

func toGraphUser(u *usersql.User) *User {
	return &User{
		ID:         u.ID,
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
	// Order by name
	UserOrderFieldName UserOrderField = "NAME"
	// Order by email
	UserOrderFieldEmail UserOrderField = "EMAIL"
)

var AllUserOrderField = []UserOrderField{
	UserOrderFieldName,
	UserOrderFieldEmail,
}

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