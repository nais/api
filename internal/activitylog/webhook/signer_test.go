package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSignPayload_Deterministic(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)

	sig1 := SignPayload("my-secret", payload)
	sig2 := SignPayload("my-secret", payload)

	if sig1 != sig2 {
		t.Errorf("expected signature to be deterministic, got %q and %q", sig1, sig2)
	}
}

func TestSignPayload_DifferentSecretsProduceDifferentSignatures(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)

	sig1 := SignPayload("secret-a", payload)
	sig2 := SignPayload("secret-b", payload)

	if sig1 == sig2 {
		t.Error("expected different secrets to produce different signatures")
	}
}

func TestSignPayload_HasSHA256Prefix(t *testing.T) {
	sig := SignPayload("my-secret", []byte("payload"))

	if !strings.HasPrefix(sig, "sha256=") {
		t.Errorf("expected signature to have sha256= prefix, got %q", sig)
	}
}

func TestSignPayload_MatchesManualComputation(t *testing.T) {
	secret := "my-secret"
	payload := []byte(`{"hello":"world"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if got := SignPayload(secret, payload); got != want {
		t.Errorf("SignPayload() = %q, want %q", got, want)
	}
}
