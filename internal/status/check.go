package status

import (
	"context"

	"github.com/nais/api/internal/workload"
)

// Check is an interface for checking the status of a workload.
// The implementation should not keep any state.
type Check interface {
	// Run the check for the given workload.
	// Returns a list of errors and the state of the workload.
	Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState)
	// Supports returns true if the check supports the given workload.
	Supports(w workload.Workload) bool
}

var checksToRun = []Check{
	checkDeprecatedIngress{},
	checkDeprecatedRegsitry{},
	checkJobRuns{},
	checkNaiserator{},
	checkNetpol{},
	checkAppNoRunningInstances{},
	checkVulnerabilities{},
}
