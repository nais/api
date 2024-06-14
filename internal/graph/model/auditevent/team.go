package auditevent

type AuditEventTeamSetAlertsSlackChannel struct {
	BaseAuditEvent
	AuditEventTeamSetAlertsSlackChannelData
}

type AuditEventTeamSetAlertsSlackChannelData struct {
	Environment string `json:"environment"`
	ChannelName string `json:"channelName"`
}

func (a AuditEventTeamSetAlertsSlackChannel) GetData() any {
	return a.AuditEventTeamSetAlertsSlackChannelData
}

func (AuditEventTeamSetAlertsSlackChannel) IsAuditEvent() {}

func (d AuditEventTeamSetAlertsSlackChannelData) String() {
}

func NewAuditEventTeamSetAlertsSlackChannel(base BaseAuditEvent, data AuditEventTeamSetAlertsSlackChannelData) *AuditEventTeamSetAlertsSlackChannel {
	return &AuditEventTeamSetAlertsSlackChannel{BaseAuditEvent: base, AuditEventTeamSetAlertsSlackChannelData: data}
}

type AuditEventTeamSetDefaultSlackChannel struct {
	BaseAuditEvent
	AuditEventTeamSetDefaultSlackChannelData
}

type AuditEventTeamSetDefaultSlackChannelData struct {
	DefaultSlackChannel string `json:"defaultSlackChannel"`
}

func (a AuditEventTeamSetDefaultSlackChannel) GetData() any {
	return a.AuditEventTeamSetDefaultSlackChannelData
}

func (AuditEventTeamSetDefaultSlackChannel) IsAuditEvent() {}

func NewAuditEventTeamSetDefaultSlackChannel(base BaseAuditEvent, data AuditEventTeamSetDefaultSlackChannelData) *AuditEventTeamSetDefaultSlackChannel {
	return &AuditEventTeamSetDefaultSlackChannel{BaseAuditEvent: base, AuditEventTeamSetDefaultSlackChannelData: data}
}

type AuditEventTeamSetPurpose struct {
	BaseAuditEvent
	AuditEventTeamSetPurposeData
}

type AuditEventTeamSetPurposeData struct {
	Purpose string `json:"purpose"`
}

func (a AuditEventTeamSetPurpose) GetData() any {
	return a.AuditEventTeamSetPurposeData
}

func (AuditEventTeamSetPurpose) IsAuditEvent() {}

func NewAuditEventTeamSetPurpose(base BaseAuditEvent, data AuditEventTeamSetPurposeData) *AuditEventTeamSetPurpose {
	return &AuditEventTeamSetPurpose{BaseAuditEvent: base, AuditEventTeamSetPurposeData: data}
}
