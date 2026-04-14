package aivencredentials

import (
	"fmt"
	"io"
	"strconv"
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

// AivenAccess converts the GraphQL enum to the string value expected by the AivenApplication CRD spec.
func (e CredentialPermission) AivenAccess() string {
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
