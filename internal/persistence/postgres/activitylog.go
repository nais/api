package postgres

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryActionDeletionRequested           activitylog.ActivityLogEntryAction = "DELETION_REQUESTED"
	activityLogEntryActionGrantAccess                 activitylog.ActivityLogEntryAction = "GRANT_ACCESS"
	activityLogEntryActionCreatePersonalAccess        activitylog.ActivityLogEntryAction = "CREATE_PERSONAL_ACCESS"
	activityLogEntryActionGetPersonalAccessConnection activitylog.ActivityLogEntryAction = "GET_PERSONAL_ACCESS_CONNECTION"
	activityLogEntryActionBranchCreated               activitylog.ActivityLogEntryAction = "BRANCH_CREATED"
	activityLogEntryActionBranchActivated             activitylog.ActivityLogEntryAction = "BRANCH_ACTIVATED"
	activityLogEntryActionBranchDeleted               activitylog.ActivityLogEntryAction = "BRANCH_DELETED"

	activityLogEntryResourceTypePostgres activitylog.ActivityLogEntryResourceType = "POSTGRES"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypePostgres, transformPostgresActivityLogEntry)

	activitylog.RegisterFilter("POSTGRES_GRANT_ACCESS", activityLogEntryActionGrantAccess, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_PERSONAL_ACCESS_CREATED", activityLogEntryActionCreatePersonalAccess, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_PERSONAL_ACCESS_CONNECTION", activityLogEntryActionGetPersonalAccessConnection, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_CREATED", activitylog.ActivityLogEntryActionCreated, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_UPDATED", activitylog.ActivityLogEntryActionUpdated, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_DELETED", activitylog.ActivityLogEntryActionDeleted, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_DELETION_REQUESTED", activityLogEntryActionDeletionRequested, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_BRANCH_CREATED", activityLogEntryActionBranchCreated, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_BRANCH_ACTIVATED", activityLogEntryActionBranchActivated, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_BRANCH_DELETED", activityLogEntryActionBranchDeleted, activityLogEntryResourceTypePostgres)
}

func transformPostgresActivityLogEntry(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
	switch entry.Action {
	case activitylog.ActivityLogEntryActionCreated:
		return PostgresCreatedActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage("Created Postgres"),
		}, nil
	case activitylog.ActivityLogEntryActionUpdated:
		data, err := activitylog.UnmarshalData[PostgresUpdatedActivityLogEntryData](entry)
		if err != nil {
			return nil, fmt.Errorf("transforming postgres updated activity log entry data: %w", err)
		}
		return PostgresUpdatedActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage("Updated Postgres"),
			Data:                    data,
		}, nil
	case activitylog.ActivityLogEntryActionDeleted:
		return PostgresDeletedActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage("Deleted Postgres branch (name unavailable)"),
		}, nil
	case activityLogEntryActionDeletionRequested:
		return PostgresDeletedActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage("Requested deletion of Postgres; cleanup is pending"),
		}, nil
	case activityLogEntryActionGrantAccess:
		if entry.TeamSlug == nil {
			return nil, fmt.Errorf("missing team slug for postgres grant access activity log entry")
		}
		if entry.EnvironmentName == nil {
			return nil, fmt.Errorf("missing environment name for postgres grant access activity log entry")
		}
		data, err := activitylog.UnmarshalData[PostgresGrantAccessActivityLogEntryData](entry)
		if err != nil {
			return nil, fmt.Errorf("transforming postgres grant access activity log entry data: %w", err)
		}
		return PostgresGrantAccessActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Granted access to %s until %s", data.Grantee, data.Until)),
			Data:                    data,
		}, nil
	case activityLogEntryActionCreatePersonalAccess:
		data, err := activitylog.UnmarshalData[PostgresPersonalAccessCreatedActivityLogEntryData](entry)
		if err != nil {
			return nil, fmt.Errorf("transforming postgres personal access activity log entry data: %w", err)
		}
		message := fmt.Sprintf("Created personal Postgres access for %s until %s", data.Username, data.ExpiresAt)
		if data.AccessLevel != nil {
			message = fmt.Sprintf("Requested %s personal Postgres access for %s until %s", *data.AccessLevel, data.Username, data.ExpiresAt)
		}
		return PostgresPersonalAccessCreatedActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage(message),
			Data:                    data,
		}, nil
	case activityLogEntryActionGetPersonalAccessConnection:
		return PostgresPersonalAccessConnectionActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage("Retrieved personal Postgres connection materials"),
		}, nil
	case activityLogEntryActionBranchCreated, activityLogEntryActionBranchActivated, activityLogEntryActionBranchDeleted:
		data, err := activitylog.UnmarshalData[PostgresBranchActivityLogEntryData](entry)
		if err != nil {
			return nil, fmt.Errorf("transforming postgres branch activity log entry data: %w", err)
		}
		switch entry.Action {
		case activityLogEntryActionBranchCreated:
			return PostgresBranchCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Postgres branch created: %s", data.Branch)),
				Data:                    data,
			}, nil
		case activityLogEntryActionBranchActivated:
			return PostgresBranchActivatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Postgres branch activated: %s", data.Branch)),
				Data:                    data,
			}, nil
		default:
			return PostgresBranchDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Postgres branch deleted: %s", data.Branch)),
				Data:                    data,
			}, nil
		}
	default:
		return nil, fmt.Errorf("unsupported postgres activity log entry action: %q", entry.Action)
	}
}

type PostgresBranchCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *PostgresBranchActivityLogEntryData `json:"data"`
}

type PostgresBranchActivatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *PostgresBranchActivityLogEntryData `json:"data"`
}

type PostgresBranchDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *PostgresBranchActivityLogEntryData `json:"data"`
}

type PostgresBranchActivityLogEntryData struct {
	Branch       string     `json:"branch"`
	SourceBranch *string    `json:"sourceBranch,omitempty"`
	TargetTime   *time.Time `json:"targetTime,omitempty"`
}

type PostgresDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type PostgresCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type PostgresUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *PostgresUpdatedActivityLogEntryData `json:"data"`
}

type PostgresUpdatedActivityLogEntryData struct {
	UpdatedFields []*PostgresUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type PostgresUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}

type PostgresGrantAccessActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *PostgresGrantAccessActivityLogEntryData `json:"data"`
}

type PostgresGrantAccessActivityLogEntryData struct {
	Grantee string    `json:"grantee,string"`
	Until   time.Time `json:"until"`
}

type PostgresPersonalAccessCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *PostgresPersonalAccessCreatedActivityLogEntryData `json:"data"`
}

type PostgresPersonalAccessCreatedActivityLogEntryData struct {
	Username    string               `json:"username"`
	AccessLevel *PostgresAccessLevel `json:"accessLevel,omitempty"`
	ExpiresAt   time.Time            `json:"expiresAt"`
	Reason      string               `json:"reason"`
}

type PostgresPersonalAccessConnectionActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
