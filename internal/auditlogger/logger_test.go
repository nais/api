package auditlogger_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

var cmpFilterPathLogs = cmp.FilterPath(func(p cmp.Path) bool {
	fields := []string{"Message", "Data", "Level"}
	if len(p) == 0 || p.String() == "" {
		return false
	}

	sf, ok := p[len(p)-1].(cmp.StructField)
	if !ok {
		return true
	}

	return !slices.Contains(fields, sf.Name())
}, cmp.Ignore())

func Test_Logf(t *testing.T) {
	ctx := context.Background()
	db := database.NewMockDatabase(t)
	msg := "some message"

	t.Run("missing audit action", func(t *testing.T) {
		testLogger, hook := test.NewNullLogger()

		auditlogger.
			New(db, testLogger).
			Logf(ctx, []auditlogger.Target{}, auditlogger.Fields{}, msg)

		want := []*logrus.Entry{
			{
				Message: "unable to create auditlog entry: missing or invalid audit action",
				Data:    logrus.Fields{},
				Level:   logrus.ErrorLevel,
			},
		}

		if diff := cmp.Diff(want, hook.AllEntries(), cmpFilterPathLogs); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})

	t.Run("does not do anything without targets", func(t *testing.T) {
		log := logrus.New()
		fields := auditlogger.Fields{
			Action: audittype.AuditActionAzureGroupAddMember,
		}
		auditlogger.
			New(db, log).
			Logf(ctx, []auditlogger.Target{}, fields, msg)
	})

	t.Run("log with target and all fields", func(t *testing.T) {
		testLogger, hook := test.NewNullLogger()

		userEmail := "mail@example.com"
		teamSlug := slug.Slug("team-slug")
		reconcilerName := "github:teams"
		systemName := "some:system"
		actorIdentity := "actor"
		action := audittype.AuditActionAzureGroupAddMember

		correlationID := uuid.New()
		targets := []auditlogger.Target{
			auditlogger.UserTarget(userEmail),
			auditlogger.TeamTarget(teamSlug),
			auditlogger.ReconcilerTarget(reconcilerName),
			auditlogger.SystemTarget(systemName),
		}

		authenticatedUser := authz.NewMockAuthenticatedUser(t)
		authenticatedUser.EXPECT().Identity().Return(actorIdentity).Once()

		fields := auditlogger.Fields{
			Action: action,
			Actor: &authz.Actor{
				User: authenticatedUser,
			},
			CorrelationID: correlationID,
		}

		db := database.NewMockDatabase(t)
		db.EXPECT().CreateAuditLogEntry(ctx, correlationID, &actorIdentity, audittype.AuditLogsTargetTypeUser, userEmail, action, msg).Return(nil).Once()
		db.EXPECT().CreateAuditLogEntry(ctx, correlationID, &actorIdentity, audittype.AuditLogsTargetTypeTeam, teamSlug.String(), action, msg).Return(nil).Once()
		db.EXPECT().CreateAuditLogEntry(ctx, correlationID, &actorIdentity, audittype.AuditLogsTargetTypeReconciler, reconcilerName, action, msg).Return(nil).Once()
		db.EXPECT().CreateAuditLogEntry(ctx, correlationID, &actorIdentity, audittype.AuditLogsTargetTypeSystem, systemName, action, msg).Return(nil).Once()

		auditlogger.
			New(db, testLogger).
			Logf(ctx, targets, fields, msg)

		want := []*logrus.Entry{
			{
				Data: logrus.Fields{
					"action":         action,
					"actor":          actorIdentity,
					"correlation_id": correlationID.String(),
					"target_type":    audittype.AuditLogsTargetTypeUser,
					"user":           "mail@example.com",
				},
				Message: msg,
				Level:   logrus.InfoLevel,
			},
			{
				Data: logrus.Fields{
					"action":         action,
					"actor":          actorIdentity,
					"correlation_id": correlationID.String(),
					"target_type":    audittype.AuditLogsTargetTypeTeam,
					"team_slug":      "team-slug",
				},
				Message: msg,
				Level:   logrus.InfoLevel,
			},
			{
				Data: logrus.Fields{
					"action":         action,
					"actor":          actorIdentity,
					"correlation_id": correlationID.String(),
					"target_type":    audittype.AuditLogsTargetTypeReconciler,
					"reconciler":     "github:teams",
				},
				Message: msg,
				Level:   logrus.InfoLevel,
			},
			{
				Data: logrus.Fields{
					"system":         systemName,
					"action":         action,
					"actor":          actorIdentity,
					"correlation_id": correlationID.String(),
					"target_type":    audittype.AuditLogsTargetTypeSystem,
				},
				Message: msg,
				Level:   logrus.InfoLevel,
			},
		}

		if diff := cmp.Diff(want, hook.AllEntries(), cmpFilterPathLogs); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})
}
