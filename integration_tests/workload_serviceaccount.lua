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

-- Binding entries resolve the workload by name, so a caller can tell an application from a job.
Test.gql("Bind service account to a workload that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "myteam"
				workloadName: "no-such-workload"
			}) {
				binding {
					isBroken
				}
			}
		}
	]], State.serviceAccountID))

	t.check {
		data = {
			addWorkloadToServiceAccount = {
				binding = {
					isBroken = true,
				},
			},
		},
	}
end)

Test.gql("Binding added entries resolve the bound workload", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				activityLog(
					first: 10
					filter: { activityTypes: [SERVICE_ACCOUNT_WORKLOAD_BINDING_ADDED] }
				) {
					nodes {
						... on ServiceAccountWorkloadBindingAddedActivityLogEntry {
							data {
								workloadName
								workload {
									__typename
									name
								}
							}
						}
					}
				}
			}
		}
	]], State.serviceAccountID))

	t.check {
		data = {
			serviceAccount = {
				activityLog = {
					nodes = {
						{
							data = {
								workloadName = "no-such-workload",
								workload = Null,
							},
						},
						{
							data = {
								workloadName = "job-running",
								workload = {
									__typename = "Job",
									name = "job-running",
								},
							},
						},
						{
							data = {
								workloadName = "app-running",
								workload = {
									__typename = "Application",
									name = "app-running",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("List bindings to find the application binding", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings(first: 10) {
					nodes {
						id
						workloadName
					}
				}
			}
		}
	]], State.serviceAccountID))

	t.check {
		data = {
			serviceAccount = {
				workloadBindings = {
					nodes = {
						{
							id = Save("appBindingID"),
							workloadName = "app-running",
						},
						{
							id = NotNull(),
							workloadName = "job-running",
						},
						{
							id = NotNull(),
							workloadName = "no-such-workload",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Remove the application binding", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			removeWorkloadFromServiceAccount(input: { bindingID: "%s" }) {
				bindingDeleted
			}
		}
	]], State.appBindingID))

	t.check {
		data = {
			removeWorkloadFromServiceAccount = {
				bindingDeleted = true,
			},
		},
	}
end)

Test.gql("Binding removed entries resolve the workload that is still deployed", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				activityLog(
					first: 10
					filter: { activityTypes: [SERVICE_ACCOUNT_WORKLOAD_BINDING_REMOVED] }
				) {
					nodes {
						... on ServiceAccountWorkloadBindingRemovedActivityLogEntry {
							data {
								workloadName
								workload {
									__typename
									name
								}
							}
						}
					}
				}
			}
		}
	]], State.serviceAccountID))

	t.check {
		data = {
			serviceAccount = {
				activityLog = {
					nodes = {
						{
							data = {
								workloadName = "app-running",
								workload = {
									__typename = "Application",
									name = "app-running",
								},
							},
						},
					},
				},
			},
		},
	}
end)
