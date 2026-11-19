package webhook

import (
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
)

func TestSubscriptionIdent_RoundTrip(t *testing.T) {
	id := uuid.New()

	got, err := parseSubscriptionIdent(newSubscriptionIdent(id))
	if err != nil {
		t.Fatalf("parseSubscriptionIdent() error = %v", err)
	}
	if got != id {
		t.Errorf("parseSubscriptionIdent() = %v, want %v", got, id)
	}
}

func TestDeliveryIdent_RoundTrip(t *testing.T) {
	id := uuid.New()

	got, err := parseDeliveryIdent(newDeliveryIdent(id))
	if err != nil {
		t.Fatalf("parseDeliveryIdent() error = %v", err)
	}
	if got != id {
		t.Errorf("parseDeliveryIdent() = %v, want %v", got, id)
	}
}

func TestParseSubscriptionIdent_MalformedIdentReturnsError(t *testing.T) {
	malformed := ident.Ident{ID: "a|b", Type: "WHS"}

	if _, err := parseSubscriptionIdent(malformed); err == nil {
		t.Error("expected an error for a malformed ident, got nil")
	}
}

func TestParseSubscriptionIdent_InvalidUUIDReturnsError(t *testing.T) {
	invalid := ident.Ident{ID: "not-a-uuid", Type: "WHS"}

	if _, err := parseSubscriptionIdent(invalid); err == nil {
		t.Error("expected an error for an invalid UUID, got nil")
	}
}

func TestParseDeliveryIdent_MalformedIdentReturnsError(t *testing.T) {
	malformed := ident.Ident{ID: "a|b", Type: "WHD"}

	if _, err := parseDeliveryIdent(malformed); err == nil {
		t.Error("expected an error for a malformed ident, got nil")
	}
}

func TestParseDeliveryIdent_InvalidUUIDReturnsError(t *testing.T) {
	invalid := ident.Ident{ID: "not-a-uuid", Type: "WHD"}

	if _, err := parseDeliveryIdent(invalid); err == nil {
		t.Error("expected an error for an invalid UUID, got nil")
	}
}
