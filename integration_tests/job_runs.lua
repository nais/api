Helper.readK8sResources("k8s_resources/job_runs")

local user = User.new()
local team = Team.new("myteam", "purpose", "#slack_channel")
team:addMember(user)

Test.gql("JobRun preserves image and manual trigger from underlying Job", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					job(name: "myjob") {
						runs {
							nodes {
								name
								image {
									name
									tag
								}
								trigger {
									type
									actor
								}
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
								{
									name = "myjob-auto",
									image = {
										name = "europe-north1-docker.pkg.dev/nais/navikt/myjob",
										tag = "v2.0.0",
									},
									trigger = {
										type = "AUTOMATIC",
										actor = "",
									},
								},
								{
									name = "myjob-manual",
									image = {
										name = "europe-north1-docker.pkg.dev/nais/navikt/myjob",
										tag = "v1.2.3",
									},
									trigger = {
										type = "MANUAL",
										actor = "user@example.com",
									},
								},
							},
						},
					},
				},
			},
		},
	}
end)
