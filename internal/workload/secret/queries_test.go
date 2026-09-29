package secret

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPersonalAccessSecretsAreNotAvailableThroughGenericSecretView(t *testing.T) {
	for _, name := range []string{"postgres-access-12345678-relay-token", "postgres-access-12345678-credentials"} {
		t.Run(name, func(t *testing.T) {
			secret := &unstructured.Unstructured{}
			secret.SetName(name)
			secret.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "nais.io/v1", Kind: "PostgresAccess", Name: "postgres-access-12345678"}})
			if !isPostgresAccessSecret(secret) {
				t.Fatal("personal access credentials must not be readable through generic Secret view")
			}
		})
	}

	ordinary := &unstructured.Unstructured{}
	ordinary.SetName("application-credentials")
	if isPostgresAccessSecret(ordinary) {
		t.Fatal("ordinary Secrets must remain readable through generic Secret view")
	}
}
