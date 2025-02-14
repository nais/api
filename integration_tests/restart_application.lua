Helper.readK8sResources("k8s_resources/simple")

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

Test.gql("as team member", function(t)
	t.query [[
		mutation {
			restartApplication(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "another-app"}
			) {
				application {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			restartApplication = {
				application = {
					name = "another-app",
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
			restartApplication(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "another-app"}
			) {
				application {
					name
				}
			}
		}
	]], { ["x-user-email"] = nonTeamMemberEmail })

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"applications:update\""),
				path = { "restartApplication" },
			},
		},
	}
end)

Test.k8s("The resource has proper annotations", function(t)
	t.check("apps/v1", "deployments", "dev", "slug-1", "another-app", {
		apiVersion = "apps/v1",
		kind = "Deployment",
		metadata = Ignore(),
		spec = {
			replicas = Ignore(),
			selector = Ignore(),
			strategy = Ignore(),
			template = {
				spec = NotNull(),
				metadata = {
					annotations = {
						["kubectl.kubernetes.io/restartedAt"] = NotNull(),
						["prometheus.io/port"] = "8080",
						["prometheus.io/scrape"] = "true",
					},
					creationTimestamp = Ignore(),
					labels = Ignore(),
				},
			},
		},
		status = Ignore(),
	})
end)
