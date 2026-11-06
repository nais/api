package webhook

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identWebhookSubscription identType = iota
	identWebhookDelivery
)

func init() {
	ident.RegisterIdentType(identWebhookSubscription, "WHS", GetSubscriptionByIdent)
	ident.RegisterIdentType(identWebhookDelivery, "WHD", GetDeliveryByIdent)
}

func newSubscriptionIdent(id uuid.UUID) ident.Ident {
	return ident.NewIdent(identWebhookSubscription, id.String())
}

func newDeliveryIdent(id uuid.UUID) ident.Ident {
	return ident.NewIdent(identWebhookDelivery, id.String())
}

func parseSubscriptionIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid webhook subscription ident")
	}

	return uuid.Parse(parts[0])
}

func parseDeliveryIdent(id ident.Ident) (uuid.UUID, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return uuid.Nil, fmt.Errorf("invalid webhook delivery ident")
	}

	return uuid.Parse(parts[0])
}
