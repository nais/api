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

Test.gql("application list", function(t)
	t.query [[
		query {
			team(slug: "slug-1") {
				applications {
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
				applications = {
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
			deleteApplication(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "%s"}
			) {
				success
			}
		}
	]], State.app1))

	t.check {
		data = {
			deleteApplication = {
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
			deleteApplication(
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
				message = Contains("you need the \"applications:delete\""),
				path = { "deleteApplication" },
			},
		},
	}
end)

Test.gql("application list after deletion", function(t)
	t.query [[
		query {
			team(slug: "slug-1") {
				applications {
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
				applications = {
					nodes = {
						{ name = "app-name" },
					},
					pageInfo = {
						totalCount = 1,
					},
				},
			},
		},
	}
end)
