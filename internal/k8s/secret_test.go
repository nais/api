package k8s

import (
	"github.com/nais/api/internal/graph/model"
	"testing"
)

func TestValidateSecretData(t *testing.T) {

	validKeys := []*model.VariableInput{
		{
			Name:  "key",
			Value: "value",
		},
		{
			Name:  "key.key-key",
			Value: "value",
		},
		{
			Name:  "key_key",
			Value: "value",
		},
		{
			Name:  "key-key",
			Value: "value",
		},
		{
			Name:  "KeyKey",
			Value: "value",
		},
	}

	err := validateSecretData(validKeys)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	invalidKeys := []*model.VariableInput{
		{
			Name:  "key@key",
			Value: "value",
		},
	}

	err = validateSecretData(invalidKeys)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	duplicateKeys := []*model.VariableInput{
		{
			Name:  "key",
			Value: "value",
		},
		{
			Name:  "key",
			Value: "value",
		},
	}

	err = validateSecretData(duplicateKeys)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}
