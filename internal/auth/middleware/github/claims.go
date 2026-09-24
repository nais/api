package github

// GitHubActorClaims holds the GitHub OIDC claims that are stored
// alongside activity log entries for audit purposes.
// See https://docs.github.com/en/actions/reference/security/oidc#oidc-token-claims.
type GitHubActorClaims struct {
	Actor                string `json:"actor"`
	ActorID              string `json:"actor_id"`
	BaseRef              string `json:"base_ref"`
	CheckRunID           string `json:"check_run_id"`
	Environment          string `json:"environment"`
	EventName            string `json:"event_name"`
	HeadRef              string `json:"head_ref"`
	JobWorkflowRef       string `json:"job_workflow_ref"`
	JobWorkflowSha       string `json:"job_workflow_sha"`
	Ref                  string `json:"ref"`
	RefType              string `json:"ref_type"`
	Repository           string `json:"repository"`
	RepositoryID         string `json:"repository_id"`
	RepositoryOwner      string `json:"repository_owner"`
	RepositoryOwnerID    string `json:"repository_owner_id"`
	RepositoryVisibility string `json:"repository_visibility"`
	RunAttempt           string `json:"run_attempt"`
	RunID                string `json:"run_id"`
	RunnerEnvironment    string `json:"runner_environment"`
	RunNumber            string `json:"run_number"`
	Workflow             string `json:"workflow"`
	WorkflowRef          string `json:"workflow_ref"`
	WorkflowSha          string `json:"workflow_sha"`
}
