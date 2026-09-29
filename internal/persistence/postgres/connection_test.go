package postgres

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestAccessSecretDataRequiresAccessOwnership(t *testing.T) {
	access := &unstructured.Unstructured{}
	access.SetName("personal-access")
	access.SetUID(types.UID("original-access"))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "personal-access-relay-token",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "nais.io/v1", Kind: "PostgresAccess", Name: "personal-access",
				UID: types.UID("original-access"), Controller: new(true),
			}},
		},
		Data: map[string][]byte{"token": []byte("private-proof")},
	}
	check := func() (string, error) {
		t.Helper()
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
		if err != nil {
			t.Fatal(err)
		}
		return accessSecretData(&unstructured.Unstructured{Object: obj}, access, "token")
	}
	if got, err := check(); err != nil || got != "private-proof" {
		t.Fatalf("owned token = %q, %v", got, err)
	}
	secret.OwnerReferences[0].UID = types.UID("replaced-access")
	if _, err := check(); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("replaced owner error = %v", err)
	}
	secret.OwnerReferences[0].UID = access.GetUID()
	delete(secret.Data, "token")
	if _, err := check(); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestRelayTokenDigest(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	sum := sha256.Sum256(raw)
	if !tokenMatchesDigest(raw, hex.EncodeToString(sum[:])) {
		t.Fatal("valid token is not accepted")
	}
	wrong := append([]byte(nil), raw...)
	wrong[0]++
	if tokenMatchesDigest(wrong, hex.EncodeToString(sum[:])) {
		t.Fatal("wrong token accepted")
	}
}
