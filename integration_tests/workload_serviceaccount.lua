Helper.readK8sResources("k8s_resources/state")

local user = User.new("authenticated", "user@user.com", "ext")
local team = Team.new("myteam", "purpose", "#channel")
team:addOwner(user)

Test.gql("Create service account for workloads", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createServiceAccount(input: {
				name: "workload-service-account"
				description: "Service account bound to test workloads"
				teamSlug: "myteam"
			}) {
				serviceAccount {
					id
				}
			}
		}
	]]

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("serviceAccountID"),
				},
			},
		},
	}
end)

Test.gql("Bind service account to application and job", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			application: addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "myteam"
				workloadName: "app-running"
			}) {
				serviceAccount {
					id
				}
			}
			job: addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "myteam"
				workloadName: "job-running"
			}) {
				serviceAccount {
					id
				}
			}
		}
	]], State.serviceAccountID, State.serviceAccountID))

	t.check {
		data = {
			application = {
				serviceAccount = {
					id = State.serviceAccountID,
				},
			},
			job = {
				serviceAccount = {
					id = State.serviceAccountID,
				},
			},
		},
	}
end)

Test.gql("Query service account through workload interface", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					application: workload(name: "app-running") {
						__typename
						serviceAccount {
							id
							name
							team {
								slug
							}
						}
					}
					job: workload(name: "job-running") {
						__typename
						serviceAccount {
							id
							name
							team {
								slug
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
					application = {
						__typename = "Application",
						serviceAccount = {
							id = State.serviceAccountID,
							name = "workload-service-account",
							team = {
								slug = "myteam",
							},
						},
					},
					job = {
						__typename = "Job",
						serviceAccount = {
							id = State.serviceAccountID,
							name = "workload-service-account",
							team = {
								slug = "myteam",
							},
						},
					},
				},
			},
		},
	}
end)
