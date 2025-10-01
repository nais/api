package aiven

import (
	aiven "github.com/aiven/go-client-codegen"
)

// IsNotFound re-exports [aiven.IsNotFound]
func IsNotFound(err error) bool {
	return aiven.IsNotFound(err)
}
