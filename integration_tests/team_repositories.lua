-- Ensure the default user has the role "Team member" for the team "slug-1"
Helper.SQLExec([[
	INSERT INTO
		user_roles (role_name, user_id, target_team_slug)
	VALUES (
		'Team member',
		(SELECT id FROM users WHERE email = 'authenticated@example.com'),
		'slug-1'
	)
	ON CONFLICT DO NOTHING;
	;
]])

Test.gql("List repositories for team", function(t)
	t.query [[
		{
			team(slug: "slug-1") {
				repositories {
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
				repositories = {
					nodes = {},
					pageInfo = {
						totalCount = 0,
					},
				},
			},
		},
	}
end)

Test.gql("Add repository to team as team member", function(t)
	t.query [[
		mutation {
			addRepositoryToTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
				repository {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			addRepositoryToTeam = {
				repository = {
					name = "nais/api",
				},
			},
		},
	}
end)

local nonTeamMemberEmail = "email-12@example.com"

Helper.SQLExec([[
	DELETE FROM user_roles WHERE user_id = (SELECT id FROM users WHERE email = $1);
]], nonTeamMemberEmail)

Test.gql("Add repository to team as non-team member", function(t)
	t.query([[
		mutation {
			addRepositoryToTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
				repository {
					name
				}
			}
		}
	]], { ["x-user-email"] = nonTeamMemberEmail })

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"repositories:create\""),
				path = { "addRepositoryToTeam" },
			},
		},
	}
end)

Test.gql("List repositories for team after creation", function(t)
	t.query [[
		{
			team(slug: "slug-1") {
				activityLog {
					nodes {
						message
						resourceName
					}
				}
				repositories {
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
				activityLog = {
					nodes = {
						{
							message = "Added repository to team",
							resourceName = "nais/api",
						},
					},
				},
				repositories = {
					nodes = {
						{
							name = "nais/api",
						},
					},
					pageInfo = {
						totalCount = 1,
					},
				},
			},
		},
	}
end)

Test.gql("Remove repository from team as non-team member", function(t)
	t.query([[
		mutation {
			removeRepositoryFromTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
				success
			}
		}
	]], { ["x-user-email"] = nonTeamMemberEmail })

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"repositories:delete\""),
				path = { "removeRepositoryFromTeam" },
			},
		},
	}
end)

Test.gql("Remove repository from team as team member", function(t)
	t.query [[
		mutation {
			removeRepositoryFromTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
				success
			}
		}
	]]

	t.check {
		data = {
			removeRepositoryFromTeam = {
				success = true,
			},
		},
	}
end)

Test.gql("List repositories for team after deletion", function(t)
	t.query [[
		{
			team(slug: "slug-1") {
				activityLog {
					nodes {
						message
						resourceName
					}
				}
				repositories {
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
				activityLog = {
					nodes = {
						{
							message = "Removed repository from team",
							resourceName = "nais/api",
						},
						{
							message = "Added repository to team",
							resourceName = "nais/api",
						},
					},
				},
				repositories = {
					nodes = {},
					pageInfo = {
						totalCount = 0,
					},
				},
			},
		},
	}
end)
