package auditlogger

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"k8s.io/utils/ptr"
)

type AuditLogger interface {
	Logf(ctx context.Context, targets []Target, fields Fields, message string, messageArgs ...interface{})
}

type auditLogger struct {
	componentName logger.ComponentName
	db            database.Database
	log           logrus.FieldLogger
}

type auditLoggerForTesting struct {
	entries []Entry
}

type Target struct {
	Type       audittype.AuditLogsTargetType
	Identifier string
}

type Fields struct {
	Action        audittype.AuditAction
	Actor         *authz.Actor
	CorrelationID uuid.UUID
}

type Entry struct {
	Context context.Context
	Targets []Target
	Fields  Fields
	Message string
}

func New(db database.Database, componentName logger.ComponentName, log logrus.FieldLogger) AuditLogger {
	return &auditLogger{
		componentName: componentName,
		db:            db,
		log:           log.WithField("component", componentName),
	}
}

func NewAuditLoggerForTesting() *auditLoggerForTesting {
	return &auditLoggerForTesting{
		entries: make([]Entry, 0),
	}
}

func (l *auditLoggerForTesting) Logf(ctx context.Context, targets []Target, fields Fields, message string, messageArgs ...interface{}) {
	l.entries = append(l.entries, Entry{
		Context: ctx,
		Targets: targets,
		Fields:  fields,
		Message: fmt.Sprintf(message, messageArgs...),
	})
}

func (l *auditLoggerForTesting) Entries() []Entry {
	return l.entries
}

// Logf Write the audit log entry to the database, and generate a system log entry. Do not call this function inside of
// a database transaction as it will generate a system log entry.
func (l *auditLogger) Logf(ctx context.Context, targets []Target, fields Fields, message string, messageArgs ...interface{}) {
	if fields.Action == "" {
		l.log.Errorf("unable to create auditlog entry: missing or invalid audit action")
		return
	}

	if fields.CorrelationID == uuid.Nil {
		fields.CorrelationID = uuid.New()
	}

	message = fmt.Sprintf(message, messageArgs...)

	var actor *string
	if fields.Actor != nil {
		actor = ptr.To[string](fields.Actor.User.Identity())
	}

	for _, target := range targets {
		err := l.db.CreateAuditLogEntry(
			ctx,
			fields.CorrelationID,
			l.componentName,
			actor,
			target.Type,
			target.Identifier,
			fields.Action,
			message,
		)
		if err != nil {
			l.log.WithError(err).Errorf("create audit log entry")
			return
		}

		logFields := logrus.Fields{
			"action":         fields.Action,
			"correlation_id": fields.CorrelationID,
			"target_type":    target.Type,
		}

		log := l.log
		if actor != nil {
			logFields["actor"] = *actor
			log = log.WithField("actor", *actor)
		}

		switch target.Type {
		case audittype.AuditLogsTargetTypeTeam:
			log = log.WithField("team_slug", target.Identifier)
		case audittype.AuditLogsTargetTypeUser:
			log = log.WithField("user", target.Identifier)
		case audittype.AuditLogsTargetTypeReconciler:
			log = log.WithField("reconciler", target.Identifier)
		default:
			logFields["target_identifier"] = target.Identifier
		}

		log.WithFields(logFields).Infof(message)
	}
}

func UserTarget(email string) Target {
	return Target{Type: audittype.AuditLogsTargetTypeUser, Identifier: email}
}

func TeamTarget(slug slug.Slug) Target {
	return Target{Type: audittype.AuditLogsTargetTypeTeam, Identifier: string(slug)}
}

func ReconcilerTarget(name sqlc.ReconcilerName) Target {
	return Target{Type: audittype.AuditLogsTargetTypeReconciler, Identifier: string(name)}
}

func ComponentTarget(name logger.ComponentName) Target {
	return Target{Type: audittype.AuditLogsTargetTypeSystem, Identifier: string(name)}
}
