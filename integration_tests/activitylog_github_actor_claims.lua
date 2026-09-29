local user = User.new()
local team = Team.new("activity-claims", "Activity claims", "#activity-claims")
team:addMember(user)

Test.sql("Store GitHub actor claims separately from activity log data", function(t)
	Helper.SQLExec([[
		INSERT INTO activity_log_entries
			(actor, action, resource_type, resource_name, team_slug, environment, created_at, data, github_actor_claims)
		VALUES
			(
				'github-repo:nais/example', 'CREATED', 'APP', 'claims-created', $1, 'dev',
				NOW() - INTERVAL '1 minute',
				CONVERT_TO('{"apiVersion":"nais.io/v1alpha1","kind":"Application"}', 'UTF8'),
				'{"actor":"octocat","repository":"nais/example","run_id":"123"}'::JSONB
			),
			(
				'github-repo:nais/example', 'UPDATED', 'APP', 'claims-updated', $1, 'dev',
				NOW() - INTERVAL '2 minutes',
				CONVERT_TO('{"changedFields":[{"field":"spec.image"}]}', 'UTF8'),
				'{"actor":"octocat","repository":"nais/example","run_id":"124"}'::JSONB
			),
			(
				'github-repo:nais/example', 'UPDATED', 'JOB', 'claims-job-updated', $1, 'dev',
				NOW() - INTERVAL '150 seconds',
				CONVERT_TO('{"changedFields":[]}', 'UTF8'),
				'{"actor":"octocat","repository":"nais/example","run_id":"126"}'::JSONB
			),
			(
				'github-repo:nais/example', 'CREATED', 'APP', 'claims-legacy', $1, 'dev',
				NOW() - INTERVAL '3 minutes',
				CONVERT_TO('{"apiVersion":"nais.io/v1alpha1","kind":"Application","gitHubActorClaims":{"actor":"octocat","repository":"nais/example","run_id":"125"}}', 'UTF8'),
				NULL
			),
			(
				'human@example.com', 'CREATED', 'APP', 'claims-absent', $1, 'dev',
				NOW() - INTERVAL '4 minutes',
				CONVERT_TO('{"apiVersion":"nais.io/v1alpha1","kind":"Application"}', 'UTF8'),
				NULL
			)
	]], team:slug())

	t.queryRow([[
		SELECT
			github_actor_claims ->> 'actor' AS actor,
			CONVERT_FROM(data, 'UTF8')::JSONB ? 'gitHubActorClaims' AS claims_in_data
		FROM activity_log_entries
		WHERE resource_name = 'claims-created'
	]])
	t.check {
		actor = "octocat",
		claims_in_data = false,
	}
end)

Test.gql("Activity log exposes stored and legacy GitHub actor claims", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		query {
			team(slug: "%s") {
				activityLog(
					first: 10
					filter: { activityTypes: [GENERIC_KUBERNETES_RESOURCE_CREATED, APPLICATION_UPDATED, JOB_UPDATED] }
				) {
					nodes {
						resourceName
						gitHubActorClaims { actor repository runID }
						... on ApplicationCreatedActivityLogEntry {
							data {
								kind
								gitHubActorClaims { actor runID }
							}
						}
						... on ApplicationUpdatedActivityLogEntry {
							data {
								changedFields { field }
								gitHubActorClaims { actor runID }
							}
						}
						... on JobUpdatedActivityLogEntry {
							data {
								gitHubActorClaims { actor runID }
							}
						}
					}
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							resourceName = "claims-created",
							gitHubActorClaims = { actor = "octocat", repository = "nais/example", runID = "123" },
							data = {
								kind = "Application",
								gitHubActorClaims = { actor = "octocat", runID = "123" },
							},
						},
						{
							resourceName = "claims-updated",
							gitHubActorClaims = { actor = "octocat", repository = "nais/example", runID = "124" },
							data = {
								changedFields = { { field = "spec.image" } },
								gitHubActorClaims = { actor = "octocat", runID = "124" },
							},
						},
						{
							resourceName = "claims-job-updated",
							gitHubActorClaims = { actor = "octocat", repository = "nais/example", runID = "126" },
							data = {
								gitHubActorClaims = { actor = "octocat", runID = "126" },
							},
						},
						{
							resourceName = "claims-legacy",
							gitHubActorClaims = { actor = "octocat", repository = "nais/example", runID = "125" },
							data = {
								kind = "Application",
								gitHubActorClaims = { actor = "octocat", runID = "125" },
							},
						},
						{
							resourceName = "claims-absent",
							gitHubActorClaims = Null,
							data = {
								kind = "Application",
								gitHubActorClaims = Null,
							},
						},
					},
				},
			},
		},
	}
end)
