package aivencredentials

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/slug"
)

// CredentialPermission represents the permission level for OpenSearch and Valkey credentials.
type CredentialPermission string

const (
	CredentialPermissionRead      CredentialPermission = "READ"
	CredentialPermissionWrite     CredentialPermission = "WRITE"
	CredentialPermissionReadWrite CredentialPermission = "READWRITE"
	CredentialPermissionAdmin     CredentialPermission = "ADMIN"
)

func (e CredentialPermission) IsValid() bool {
	switch e {
	case CredentialPermissionRead, CredentialPermissionWrite, CredentialPermissionReadWrite, CredentialPermissionAdmin:
		return true
	}
	return false
}

func (e CredentialPermission) String() string {
	return string(e)
}

func (e *CredentialPermission) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = CredentialPermission(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid CredentialPermission", str)
	}
	return nil
}

func (e CredentialPermission) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// aivenAccess converts the GraphQL enum to the string value expected by the AivenApplication CRD spec.
func (e CredentialPermission) aivenAccess() string {
	switch e {
	case CredentialPermissionRead:
		return "read"
	case CredentialPermissionWrite:
		return "write"
	case CredentialPermissionReadWrite:
		return "readwrite"
	case CredentialPermissionAdmin:
		return "admin"
	default:
		return "read"
	}
}

// Input types

type CreateOpenSearchCredentialsInput struct {
	TeamSlug        slug.Slug            `json:"teamSlug"`
	EnvironmentName string               `json:"environmentName"`
	InstanceName    string               `json:"instanceName"`
	Permission      CredentialPermission `json:"permission"`
	TTL             string               `json:"ttl"`
}

type CreateValkeyCredentialsInput struct {
	TeamSlug        slug.Slug            `json:"teamSlug"`
	EnvironmentName string               `json:"environmentName"`
	InstanceName    string               `json:"instanceName"`
	Permission      CredentialPermission `json:"permission"`
	TTL             string               `json:"ttl"`
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
