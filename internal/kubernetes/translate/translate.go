// Package translate translates Kubernetes status messages into Nais wording.
package translate

import (
	"fmt"
	"regexp"
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

var (
	regMissingSecret = regexp.MustCompile(`(?i)secrets? "([^"]+)" not found`)
	regMissingConfig = regexp.MustCompile(`(?i)configmaps? "([^"]+)" not found`)
	regMissingKey    = regexp.MustCompile(`couldn't find key (\S+) in (Secret|ConfigMap) (?:[^/\s]+/)?(\S+)`)
)

// MissingSource describes a missing Secret or Config referenced by a workload, if the message mentions one.
func MissingSource(message string) (string, bool) {
	if m := regMissingKey.FindStringSubmatch(message); m != nil {
		kind := "secret"
		if m[2] == "ConfigMap" {
			kind = "config"
		}
		return fmt.Sprintf("Key %q was not found in %s %q.", m[1], kind, m[3]), true
	}
	if m := regMissingSecret.FindStringSubmatch(message); m != nil {
		return fmt.Sprintf("Secret %q was not found.", m[1]), true
	}
	if m := regMissingConfig.FindStringSubmatch(message); m != nil {
		return fmt.Sprintf("Config %q was not found.", m[1]), true
	}
	return "", false
}
