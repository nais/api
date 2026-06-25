local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addOwner(user)

Test.gql("Create service account for team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "binding-sa"
					description: "service account for workload binding tests"
					teamSlug: "slug-1"
				}
			) {
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
					id = Save("saID"),
				},
			},
		},
	}
end)

Test.gql("Add workload to service account as non-member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "slug-1"
				workloadName: "my-app"
			}) {
				binding {
					id
				}
			}
		}
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("Specifically, you need the \"service_accounts:update\" authorization."),
				path = {
					"addWorkloadToServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Add workload to service account as member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "slug-1"
				workloadName: "my-app"
			}) {
				serviceAccount {
					id
				}
				binding {
					id
					environment
					teamSlug
					workloadName
					isBroken
					lastUsedAt
					workload {
						__typename
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			addWorkloadToServiceAccount = {
				serviceAccount = {
					id = State.saID,
				},
				binding = {
					id = Save("bindingID"),
					environment = "dev",
					teamSlug = "slug-1",
					workloadName = "my-app",
					-- The workload doesn't actually exist in the fake cluster, so the binding is reported as
					-- broken and the workload is null.
					isBroken = true,
					lastUsedAt = Null,
					workload = Null,
				},
			},
		},
	}
end)

Test.gql("Add same workload again returns error", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "slug-1"
				workloadName: "my-app"
			}) {
				binding {
					id
				}
			}
		}
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("already bound to service account"),
				path = {
					"addWorkloadToServiceAccount",
				},
			},
		},
	}
end)

Test.gql("List workload bindings on service account", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings {
					nodes {
						id
						environment
						teamSlug
						workloadName
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			serviceAccount = {
				workloadBindings = {
					nodes = {
						{
							id = State.bindingID,
							environment = "dev",
							teamSlug = "slug-1",
							workloadName = "my-app",
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

Test.gql("Remove workload binding as non-member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query(string.format([[
		mutation {
			removeWorkloadFromServiceAccount(input: { bindingID: "%s" }) {
				bindingDeleted
			}
		}
	]], State.bindingID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("Specifically, you need the \"service_accounts:update\" authorization."),
				path = {
					"removeWorkloadFromServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Remove workload binding as member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			removeWorkloadFromServiceAccount(input: { bindingID: "%s" }) {
				serviceAccount {
					id
				}
				bindingDeleted
			}
		}
	]], State.bindingID))

	t.check {
		data = {
			removeWorkloadFromServiceAccount = {
				serviceAccount = {
					id = State.saID,
				},
				bindingDeleted = true,
			},
		},
	}
end)

Test.gql("Remove non-existent workload binding", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			removeWorkloadFromServiceAccount(input: { bindingID: "%s" }) {
				bindingDeleted
			}
		}
	]], State.bindingID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("service account workload binding was not found"),
				path = {
					"removeWorkloadFromServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Empty workload bindings after removal", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings {
					pageInfo {
						totalCount
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			serviceAccount = {
				workloadBindings = {
					pageInfo = {
						totalCount = 0,
					},
				},
			},
		},
	}
end)
