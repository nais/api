package kafkatopic

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogEntryResourceTypeKafkaTopic activitylog.ActivityLogEntryResourceType = "KAFKA_TOPIC"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeKafkaTopic, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.UnmarshalData[KafkaTopicUpdatedActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Kafka topic updated activity log entry data: %w", err)
			}

			return KafkaTopicUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated Kafka topic"),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionCredentialsCreated:
			data, err := activitylog.UnmarshalData[KafkaCredentialsCreatedActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Kafka credentials creation activity log entry data: %w", err)
			}

			msg := fmt.Sprintf("Created credentials for %q", entry.ResourceName)
			msg += fmt.Sprintf(" (TTL: %s)", data.TTL)

			return KafkaCredentialsCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(msg),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported kafka topic activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("KAFKA_CREDENTIALS_CREATED", activitylog.ActivityLogEntryActionCredentialsCreated, ActivityLogEntryResourceTypeKafkaTopic)
	activitylog.RegisterFilter("KAFKA_TOPIC_UPDATED", activitylog.ActivityLogEntryActionUpdated, ActivityLogEntryResourceTypeKafkaTopic)
}

type KafkaTopicUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *KafkaTopicUpdatedActivityLogEntryData `json:"data"`
}

type KafkaTopicUpdatedActivityLogEntryData struct {
	AddedGrants   []KafkaTopicUpdatedActivityLogEntryDataGrant `json:"addedGrants"`
	RevokedGrants []KafkaTopicUpdatedActivityLogEntryDataGrant `json:"revokedGrants"`
}

type KafkaTopicUpdatedActivityLogEntryDataGrant struct {
	Subject  string                `json:"subject"`
	TeamName string                `json:"teamName"`
	Access   KafkaTopicGrantAccess `json:"access"`
}

type KafkaCredentialsCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *KafkaCredentialsCreatedActivityLogEntryData `json:"data"`
}

type KafkaCredentialsCreatedActivityLogEntryData struct {
	TTL string `json:"ttl"`
}
