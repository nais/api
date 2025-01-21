package usersync

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "USLE", getByIdent)
}

func newIdent(uid uuid.UUID) ident.Ident {
	return ident.NewIdent(identKey, base58.Encode(uid[:]))
}

func parseIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid user sync log entry ident")
	}

	return uuid.FromBytes(base58.Decode(parts[0]))
}

func getByIdent(ctx context.Context, id ident.Ident) (UserSyncLogEntry, error) {
	uid, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return fromContext(ctx).userSyncLogLoader.Load(ctx, uid)
}
