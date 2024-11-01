package scalar

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

type IdentType string

const (
	IdentTypeTeam        IdentType = "team"
	IdentTypeUser        IdentType = "user"
	IdentTypeUsersyncRun IdentType = "usersyncRun"

	idSeparator = "-"
)

type Ident struct {
	ID   string
	Type IdentType
}

func (i Ident) AsUUID() (uuid.UUID, error) {
	return uuid.Parse(i.ID)
}

func (i Ident) MarshalGQLContext(_ context.Context, w io.Writer) error {
	if i.ID == "" || i.Type == "" {
		return fmt.Errorf("id and type must be set")
	}
	v := url.Values{}
	v.Set("id", i.ID)
	v.Set("type", string(i.Type))
	_, err := w.Write([]byte(strconv.Quote(base64.URLEncoding.EncodeToString([]byte(v.Encode())))))
	return err
}

func (i *Ident) UnmarshalGQLContext(_ context.Context, v interface{}) error {
	ident, ok := v.(string)
	if !ok {
		return fmt.Errorf("ident must be a string")
	}

	bytes, err := base64.URLEncoding.DecodeString(ident)
	if err != nil {
		return err
	}

	values, err := url.ParseQuery(string(bytes))
	if err != nil {
		return err
	}

	i.ID = values.Get("id")
	i.Type = IdentType(values.Get("type"))

	return nil
}

func TeamIdent(teamSlug slug.Slug) Ident {
	return newIdent(IdentTypeTeam, string(teamSlug))
}

func UserIdent(userID uuid.UUID) Ident {
	return newIdent(IdentTypeUser, userID.String())
}

func UsersyncRunIdent(id uuid.UUID) Ident {
	return newIdent(IdentTypeUsersyncRun, id.String())
}

func newIdent(t IdentType, id ...string) Ident {
	return Ident{
		ID:   strings.Join(id, idSeparator),
		Type: t,
	}
}
