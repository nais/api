package grpcuser

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/pkg/apiclient/protoapi"
)

func ToProtoUser(user *database.User) *protoapi.User {
	return &protoapi.User{
		Id:         user.ID.String(),
		Name:       user.Name,
		Email:      user.Email,
		ExternalId: user.ExternalID,
	}
}
