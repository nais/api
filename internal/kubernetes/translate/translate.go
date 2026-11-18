// Package translate translates Kubernetes status messages into Nais wording.
package translate

import (
	"fmt"
	"strings"
)

// SchedulingFailure describes why an instance could not be placed on a server, based on a scheduling failure
// message from Kubernetes.
func SchedulingFailure(message string) string {
	var resources []string
	if strings.Contains(message, "Insufficient cpu") {
		resources = append(resources, "CPU")
	}
	if strings.Contains(message, "Insufficient memory") {
		resources = append(resources, "memory")
	}
	if len(resources) > 0 {
		return fmt.Sprintf("Not enough %s available to start the instance. Consider lowering the requested resources.", strings.Join(resources, " and "))
	}
	return "No suitable server was found to run the instance."
}
