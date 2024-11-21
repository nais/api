Helper.readK8sResources("k8s_resources/simple")

-- Ensure the default user has the role "Team member" for the team "slug-1"
Helper.SQLExec([[
	INSERT INTO
		user_roles (role_name, user_id, target_team_slug)
	VALUES (
		'Team member'::role_name,
		(SELECT id FROM users WHERE email = 'authenticated@example.com'),
		'slug-1'
	)
	ON CONFLICT DO NOTHING;
	;
]])

Test.gql("job list", function(t)
	t.query [[
		query {
			team(slug: "slug-1") {
				jobs {
					nodes {
						name
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				jobs = {
					nodes = {
						{ name = Save("app1") },
						{ name = Save("app2") },
					},
					pageInfo = {
						totalCount = 2,
					},
				},
			},
		},
	}
end)

Test.gql("as team member", function(t)
	t.query(string.format([[
		mutation {
			deleteJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "%s"}
			) {
				success
			}
		}
	]], State.app1))

	t.check {
		data = {
			deleteJob = {
				success = true,
			},
		},
	}
end)

local nonTeamMemberEmail = "email-12@example.com"

Helper.SQLExec([[
	DELETE FROM user_roles WHERE user_id = (SELECT id FROM users WHERE email = $1);
]], nonTeamMemberEmail)

Test.gql("as non-team member", function(t)
	t.query(string.format([[
		mutation {
			deleteJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "%s"}
			) {
				success
			}
		}
	]], State.app2), { ["x-user-email"] = nonTeamMemberEmail })

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"jobs:delete\""),
				path = { "deleteJob" },
			},
		},
	}
end)

Test.gql("job list after deletion", function(t)
	t.query [[
		query {
			team(slug: "slug-1") {
				jobs {
					nodes {
						name
					}
					pageInfo {
						totalCount
					}
				}
				auditEntries {
					nodes {
						message
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				auditEntries = {
					nodes = {
						{
							message = "Job deleted",
						},
					},
				},
				jobs = {
					nodes = {
						{ name = "jobname-2" },
					},
					pageInfo = {
						totalCount = 1,
					},
				},
			},
		},
	}
end)
