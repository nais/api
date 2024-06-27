package auditevent

import (
	"github.com/nais/api/internal/graph/model"
)

type AuditEventTeamAddRepository struct {
	BaseTeamAuditEvent
	Data model.AuditEventTeamAddRepositoryData
}

func (a AuditEventTeamAddRepository) GetData() any {
	return a.Data
}

type AuditEventTeamRemoveRepository struct {
	BaseTeamAuditEvent
	Data model.AuditEventTeamRemoveRepositoryData
}

func (a AuditEventTeamRemoveRepository) GetData() any {
	return a.Data
}
