package github

// GitHubActorClaims holds the GitHub OIDC claims that are stored
// alongside activity log entries for audit purposes.
// See https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims.
type GitHubActorClaims struct {
	Actor                string  `json:"actor"`
	ActorID              *string `json:"actor_id,omitempty"`
	BaseRef              *string `json:"base_ref,omitempty"`
	CheckRunID           *string `json:"check_run_id,omitempty"`
	Environment          string  `json:"environment"`
	EventName            string  `json:"event_name"`
	HeadRef              *string `json:"head_ref,omitempty"`
	JobWorkflowRef       string  `json:"job_workflow_ref"`
	JobWorkflowSha       *string `json:"job_workflow_sha,omitempty"`
	Ref                  string  `json:"ref"`
	RefType              *string `json:"ref_type,omitempty"`
	Repository           string  `json:"repository"`
	RepositoryID         string  `json:"repository_id"`
	RepositoryOwner      *string `json:"repository_owner,omitempty"`
	RepositoryOwnerID    *string `json:"repository_owner_id,omitempty"`
	RepositoryVisibility *string `json:"repository_visibility,omitempty"`
	RunAttempt           string  `json:"run_attempt"`
	RunID                string  `json:"run_id"`
	RunnerEnvironment    *string `json:"runner_environment,omitempty"`
	RunNumber            *string `json:"run_number,omitempty"`
	Workflow             string  `json:"workflow"`
	WorkflowRef          *string `json:"workflow_ref,omitempty"`
	WorkflowSha          *string `json:"workflow_sha,omitempty"`
}
