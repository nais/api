package aivencredentials

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/slug"
)

// AivenPermission represents the permission level for OpenSearch and Valkey credentials.
type AivenPermission string

const (
	AivenPermissionRead      AivenPermission = "READ"
	AivenPermissionWrite     AivenPermission = "WRITE"
	AivenPermissionReadWrite AivenPermission = "READWRITE"
	AivenPermissionAdmin     AivenPermission = "ADMIN"
)

func (e AivenPermission) IsValid() bool {
	switch e {
	case AivenPermissionRead, AivenPermissionWrite, AivenPermissionReadWrite, AivenPermissionAdmin:
		return true
	}
	return false
}

func (e AivenPermission) String() string {
	return string(e)
}

func (e *AivenPermission) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = AivenPermission(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid AivenPermission", str)
	}
	return nil
}

func (e AivenPermission) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// aivenAccess converts the GraphQL enum to the string value expected by the AivenApplication CRD spec.
func (e AivenPermission) aivenAccess() string {
	switch e {
	case AivenPermissionRead:
		return "read"
	case AivenPermissionWrite:
		return "write"
	case AivenPermissionReadWrite:
		return "readwrite"
	case AivenPermissionAdmin:
		return "admin"
	default:
		return "read"
	}
}

// KafkaPermission represents the permission level for Kafka credentials.
type KafkaPermission string

const (
	KafkaPermissionRead      KafkaPermission = "READ"
	KafkaPermissionWrite     KafkaPermission = "WRITE"
	KafkaPermissionReadWrite KafkaPermission = "READWRITE"
)

func (e KafkaPermission) IsValid() bool {
	switch e {
	case KafkaPermissionRead, KafkaPermissionWrite, KafkaPermissionReadWrite:
		return true
	}
	return false
}

func (e KafkaPermission) String() string {
	return string(e)
}

func (e *KafkaPermission) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = KafkaPermission(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid KafkaPermission", str)
	}
	return nil
}

func (e KafkaPermission) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// Input types

type CreateOpenSearchCredentialsInput struct {
	TeamSlug        slug.Slug       `json:"teamSlug"`
	EnvironmentName string          `json:"environmentName"`
	InstanceName    string          `json:"instanceName"`
	Permission      AivenPermission `json:"permission"`
	TTL             string          `json:"ttl"`
}

type CreateValkeyCredentialsInput struct {
	TeamSlug        slug.Slug       `json:"teamSlug"`
	EnvironmentName string          `json:"environmentName"`
	InstanceName    string          `json:"instanceName"`
	Permission      AivenPermission `json:"permission"`
	TTL             string          `json:"ttl"`
}

type CreateKafkaCredentialsInput struct {
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
	TTL             string    `json:"ttl"`
}

// Credential types

type OpenSearchCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	URI      string `json:"uri"`
}

type ValkeyCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	URI      string `json:"uri"`
}

type KafkaCredentials struct {
	Username       string `json:"username"`
	AccessCert     string `json:"accessCert"`
	AccessKey      string `json:"accessKey"`
	CaCert         string `json:"caCert"`
	Brokers        string `json:"brokers"`
	SchemaRegistry string `json:"schemaRegistry"`
}

// Payload types

type CreateOpenSearchCredentialsPayload struct {
	Credentials *OpenSearchCredentials `json:"credentials"`
}

type CreateValkeyCredentialsPayload struct {
	Credentials *ValkeyCredentials `json:"credentials"`
}

type CreateKafkaCredentialsPayload struct {
	Credentials *KafkaCredentials `json:"credentials"`
}
