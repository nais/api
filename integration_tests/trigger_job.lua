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

Test.gql("job details", function(t)
	t.query [[
		{
			team(slug: "slug-1") {
				environment(name: "dev") {
					job(name: "jobname-1") {
						runs {
							nodes {
								name
							}
							pageInfo {
								totalCount
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				environment = {
					job = {
						runs = {
							nodes = {
								{ name = "jobname-1-run1" },
							},
							pageInfo = {
								totalCount = 1,
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("as team member", function(t)
	t.query [[
		mutation {
			triggerJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "jobname-1", runName: "newRun"}
			) {
				jobRun {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			triggerJob = {
				jobRun = {
					name = "newRun",
				},
			},
		},
	}
end)

local nonTeamMemberEmail = "email-12@example.com"

Helper.SQLExec([[
	DELETE FROM user_roles WHERE user_id = (SELECT id FROM users WHERE email = $1);
]], nonTeamMemberEmail)

Test.gql("as non-team member", function(t)
	t.query([[
		mutation {
			triggerJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "jobname-1", runName: "newRun2"}
			) {
				jobRun {
					name
				}
			}
		}
	]], { ["x-user-email"] = nonTeamMemberEmail })

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"jobs:update\""),
				path = { "triggerJob" },
			},
		},
	}
end)


Test.gql("job details after trigger", function(t)
	t.query [[
		{
			team(slug: "slug-1") {
				auditEntries {
					nodes {
						message
					}
				}
				environment(name: "dev") {
					job(name: "jobname-1") {
						runs {
							nodes {
								name
							}
							pageInfo {
								totalCount
							}
						}
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
							message = "Job triggered",
						},
					},
				},
				environment = {
					job = {
						runs = {
							nodes = {
								{ name = "jobname-1-run1" },
								{ name = "newRun" },
							},
							pageInfo = {
								totalCount = 2,
							},
						},
					},
				},
			},
		},
	}
end)
