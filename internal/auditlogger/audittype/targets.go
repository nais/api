package audittype

type AuditLogsTargetType string

const (
	AuditLogsTargetTypeReconciler     AuditLogsTargetType = "reconciler"
	AuditLogsTargetTypeServiceAccount AuditLogsTargetType = "service_account"
	AuditLogsTargetTypeSystem         AuditLogsTargetType = "system"
	AuditLogsTargetTypeTeam           AuditLogsTargetType = "team"
	AuditLogsTargetTypeUser           AuditLogsTargetType = "user"
)
