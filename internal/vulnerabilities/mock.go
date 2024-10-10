package vulnerabilities

import dependencytrack "github.com/nais/dependencytrack/pkg/client"

type InternalClient interface {
	dependencytrack.Client
}
