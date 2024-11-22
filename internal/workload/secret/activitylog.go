package secret

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	auditResourceTypeSecret      activitylog.AuditResourceType = "SECRET"
	auditActionAddSecretValue    activitylog.AuditAction       = "ADD_SECRET_VALUE"
	auditActionUpdateSecretValue                               = "UPDATE_SECRET_VALUE"
	auditActionRemoveSecretValue                               = "REMOVE_SECRET_VALUE"
)

func init() {
	activitylog.RegisterTransformer(auditResourceTypeSecret, func(entry activitylog.GenericAuditEntry) (activitylog.AuditEntry, error) {
		switch entry.Action {
		case activitylog.AuditActionCreated:
			return SecretCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created secret"),
			}, nil
		case activitylog.AuditActionDeleted:
			return SecretDeletedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Deleted secret"),
			}, nil
		case auditActionAddSecretValue:
			data, err := activitylog.TransformData(entry, func(data *SecretValueAddedAuditEntryData) *SecretValueAddedAuditEntryData {
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
			data, err := activitylog.TransformData(entry, func(data *SecretValueUpdatedAuditEntryData) *SecretValueUpdatedAuditEntryData {
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
			data, err := activitylog.TransformData(entry, func(data *SecretValueRemovedAuditEntryData) *SecretValueRemovedAuditEntryData {
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
	activitylog.GenericAuditEntry
}

type SecretValueAddedAuditEntry struct {
	activitylog.GenericAuditEntry
	Data *SecretValueAddedAuditEntryData
}

type SecretValueAddedAuditEntryData struct {
	ValueName string
}

type SecretValueUpdatedAuditEntry struct {
	activitylog.GenericAuditEntry
	Data *SecretValueUpdatedAuditEntryData
}

type SecretValueUpdatedAuditEntryData struct {
	ValueName string
}

type SecretValueRemovedAuditEntry struct {
	activitylog.GenericAuditEntry
	Data *SecretValueRemovedAuditEntryData
}

type SecretValueRemovedAuditEntryData struct {
	ValueName string
}

type SecretDeletedAuditEntry struct {
	activitylog.GenericAuditEntry
}
