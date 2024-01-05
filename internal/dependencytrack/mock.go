package dependencytrack

import "github.com/nais/dependencytrack/pkg/client"

type InternalClient interface {
	client.Client
}
