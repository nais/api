package secret

import (
	"fmt"

	"github.com/nais/api/internal/v1/auditv1"
)

const (
	auditResourceTypeSecret      auditv1.AuditResourceType = "SECRET"
	auditActionCreateSecret      auditv1.AuditAction       = "CREATE_SECRET"
	auditActionDeleteSecret                                = "DELETE_SECRET"
	auditActionAddSecretValue                              = "ADD_SECRET_VALUE"
	auditActionUpdateSecretValue                           = "UPDATE_SECRET_VALUE"
	auditActionRemoveSecretValue                           = "REMOVE_SECRET_VALUE"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeSecret, func(entry auditv1.GenericAuditEntry) (auditv1.AuditEntry, error) {
		switch entry.Action {
		case auditActionCreateSecret:
			return SecretCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created secret"),
			}, nil
		case auditActionDeleteSecret:
			return SecretDeletedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Deleted secret"),
			}, nil
		case auditActionAddSecretValue:
			data, err := auditv1.TransformData(entry, func(data *SecretValueAddedAuditEntryData) *SecretValueAddedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueAddedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Added secret value"),
				Data:              data,
			}, nil
		case auditActionUpdateSecretValue:
			data, err := auditv1.TransformData(entry, func(data *SecretValueUpdatedAuditEntryData) *SecretValueUpdatedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Updated secret value"),
				Data:              data,
			}, nil
		case auditActionRemoveSecretValue:
			data, err := auditv1.TransformData(entry, func(data *SecretValueRemovedAuditEntryData) *SecretValueRemovedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return SecretValueRemovedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Removed secret value"),
				Data:              data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported secret audit entry action: %q", entry.Action)
		}
	})
}

type SecretCreatedAuditEntry struct {
	auditv1.GenericAuditEntry
}

type SecretValueAddedAuditEntry struct {
	auditv1.GenericAuditEntry
	Data *SecretValueAddedAuditEntryData
}

type SecretValueAddedAuditEntryData struct {
	ValueName string
}

type SecretValueUpdatedAuditEntry struct {
	auditv1.GenericAuditEntry
	Data *SecretValueUpdatedAuditEntryData
}

type SecretValueUpdatedAuditEntryData struct {
	ValueName string
}

type SecretValueRemovedAuditEntry struct {
	auditv1.GenericAuditEntry
	Data *SecretValueRemovedAuditEntryData
}

type SecretValueRemovedAuditEntryData struct {
	ValueName string
}

type SecretDeletedAuditEntry struct {
	auditv1.GenericAuditEntry
}
