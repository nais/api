local user = User.new()
local nonMember = User.new()
local team1 = Team.new("team-alpha", "alpha purpose", "#alpha")
local team2 = Team.new("team-beta", "beta purpose", "#beta")
team1:addOwner(user)
team2:addOwner(user)

-- Create service account on team-alpha
Test.gql("Create service account on team-alpha", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "full-binding-sa"
					description: "service account for full workload binding tests"
					teamSlug: "team-alpha"
				}
			) {
				serviceAccount {
					id
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("saID"),
					name = "full-binding-sa",
				},
			},
		},
	}
end)

-- Create a dummy SA, delete it, and use the stale ID for error tests later
Test.gql("Create dummy SA for non-existent ID tests", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "dummy-binding-sa"
					description: "will be deleted"
					teamSlug: "team-alpha"
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
					id = Save("deletedSaID"),
				},
			},
		},
	}
end)

Test.gql("Delete dummy SA", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			deleteServiceAccount(input: { serviceAccountID: "%s" }) {
				serviceAccountDeleted
			}
		}
	]], State.deletedSaID))

	t.check {
		data = {
			deleteServiceAccount = {
				serviceAccountDeleted = true,
			},
		},
	}
end)

-- Test 1: Full field coverage on binding type
Test.gql("Add binding and verify all fields", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "team-alpha"
				workloadName: "app-one"
			}) {
				serviceAccount {
					id
					name
				}
				binding {
					id
					environment
					teamSlug
					workloadName
					isBroken
					createdAt
					lastUsedAt
					workload {
						__typename
					}
					serviceAccount {
						id
						name
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
					name = "full-binding-sa",
				},
				binding = {
					id = Save("binding1ID"),
					environment = "dev",
					teamSlug = "team-alpha",
					workloadName = "app-one",
					isBroken = true,
					createdAt = NotNull(),
					lastUsedAt = Null,
					workload = Null,
					serviceAccount = {
						id = State.saID,
						name = "full-binding-sa",
					},
				},
			},
		},
	}
end)

-- Test 2: Verify serviceAccount field resolves correctly via query
Test.gql("Query binding and verify serviceAccount resolves", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings(first: 10) {
					nodes {
						id
						serviceAccount {
							id
							name
						}
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
							id = State.binding1ID,
							serviceAccount = {
								id = State.saID,
								name = "full-binding-sa",
							},
						},
					},
				},
			},
		},
	}
end)

-- Test 3: Add workload binding to a non-existent (deleted) service account
Test.gql("Add binding to non-existent service account", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "team-alpha"
				workloadName: "my-app"
			}) {
				binding {
					id
				}
			}
		}
	]], State.deletedSaID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "The specified service account was not found.",
				path = {
					"addWorkloadToServiceAccount",
				},
			},
		},
	}
end)

-- Test 4: Add workload binding with empty workload name
Test.gql("Add binding with empty workload name", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "team-alpha"
				workloadName: ""
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
				message = Contains("Workload name must not be empty"),
				path = {
					"addWorkloadToServiceAccount",
				},
			},
		},
	}
end)

-- Test 5: Add bindings in different environments for pagination
Test.gql("Add binding in staging environment", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "staging"
				teamSlug: "team-alpha"
				workloadName: "app-two"
			}) {
				binding {
					id
					environment
					teamSlug
					workloadName
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			addWorkloadToServiceAccount = {
				binding = {
					id = Save("binding2ID"),
					environment = "staging",
					teamSlug = "team-alpha",
					workloadName = "app-two",
				},
			},
		},
	}
end)

Test.gql("Add binding in dev-gcp environment", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev-gcp"
				teamSlug: "team-alpha"
				workloadName: "app-three"
			}) {
				binding {
					id
					environment
					teamSlug
					workloadName
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			addWorkloadToServiceAccount = {
				binding = {
					id = Save("binding3ID"),
					environment = "dev-gcp",
					teamSlug = "team-alpha",
					workloadName = "app-three",
				},
			},
		},
	}
end)

-- Test 6: Add binding referencing a different team slug
Test.gql("Add binding referencing team-beta", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "team-beta"
				workloadName: "beta-app"
			}) {
				binding {
					id
					environment
					teamSlug
					workloadName
					isBroken
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			addWorkloadToServiceAccount = {
				binding = {
					id = Save("binding4ID"),
					environment = "dev",
					teamSlug = "team-beta",
					workloadName = "beta-app",
					isBroken = true,
				},
			},
		},
	}
end)

-- Test 7: Duplicate binding (same environment + team + workload) returns error
Test.gql("Add duplicate binding returns error", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "team-alpha"
				workloadName: "app-one"
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
				message = Contains("already bound to a service account"),
				path = {
					"addWorkloadToServiceAccount",
				},
			},
		},
	}
end)

-- Test 8: Verify pagination totalCount with all bindings
Test.gql("Verify pagination totalCount with all bindings", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings {
					pageInfo {
						totalCount
						hasNextPage
						hasPreviousPage
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
						totalCount = 4,
						hasNextPage = false,
						hasPreviousPage = false,
					},
				},
			},
		},
	}
end)

-- Test 9: Verify pagination with first/after
Test.gql("Verify pagination with first:2", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings(first: 2) {
					pageInfo {
						totalCount
						hasNextPage
						hasPreviousPage
						endCursor
					}
					nodes {
						id
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
						totalCount = 4,
						hasNextPage = true,
						hasPreviousPage = false,
						endCursor = Save("page1EndCursor"),
					},
					nodes = NotNull(),
				},
			},
		},
	}
end)

Test.gql("Verify pagination with first:2 after cursor", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings(first: 2, after: "%s") {
					pageInfo {
						totalCount
						hasNextPage
						hasPreviousPage
					}
					nodes {
						id
					}
				}
			}
		}
	]], State.saID, State.page1EndCursor))

	t.check {
		data = {
			serviceAccount = {
				workloadBindings = {
					pageInfo = {
						totalCount = 4,
						hasNextPage = false,
						hasPreviousPage = true,
					},
					nodes = NotNull(),
				},
			},
		},
	}
end)

-- Test 10: Remove one binding and verify count decreases
Test.gql("Remove binding for app-three", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			removeWorkloadFromServiceAccount(input: { bindingID: "%s" }) {
				serviceAccount {
					id
					workloadBindings {
						pageInfo {
							totalCount
						}
					}
				}
				bindingDeleted
			}
		}
	]], State.binding3ID))

	t.check {
		data = {
			removeWorkloadFromServiceAccount = {
				serviceAccount = {
					id = State.saID,
					workloadBindings = {
						pageInfo = {
							totalCount = 3,
						},
					},
				},
				bindingDeleted = true,
			},
		},
	}
end)

-- Test 11: Verify removed binding cannot be removed again
Test.gql("Remove already removed binding returns error", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			removeWorkloadFromServiceAccount(input: { bindingID: "%s" }) {
				bindingDeleted
			}
		}
	]], State.binding3ID))

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

-- Test 12: Verify remaining bindings are intact
Test.gql("Verify remaining bindings after removal", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings {
					pageInfo {
						totalCount
					}
					nodes {
						id
						environment
						teamSlug
						workloadName
						isBroken
						createdAt
						lastUsedAt
						workload {
							__typename
						}
						serviceAccount {
							id
						}
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
						totalCount = 3,
					},
					nodes = NotNull(),
				},
			},
		},
	}
end)

-- Test 13: Create a second service account and verify bindings are isolated
Test.gql("Create second service account on team-beta", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "beta-sa"
					description: "service account for team beta"
					teamSlug: "team-beta"
				}
			) {
				serviceAccount {
					id
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("sa2ID"),
					name = "beta-sa",
				},
			},
		},
	}
end)

Test.gql("Add binding to second service account", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "team-beta"
				workloadName: "isolated-app"
			}) {
				binding {
					id
					environment
					teamSlug
					workloadName
					serviceAccount {
						id
						name
					}
				}
			}
		}
	]], State.sa2ID))

	t.check {
		data = {
			addWorkloadToServiceAccount = {
				binding = {
					id = Save("binding5ID"),
					environment = "dev",
					teamSlug = "team-beta",
					workloadName = "isolated-app",
					serviceAccount = {
						id = State.sa2ID,
						name = "beta-sa",
					},
				},
			},
		},
	}
end)

Test.gql("Verify second SA bindings are isolated from first SA", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				workloadBindings {
					pageInfo {
						totalCount
					}
					nodes {
						id
						workloadName
						serviceAccount {
							id
						}
					}
				}
			}
		}
	]], State.sa2ID))

	t.check {
		data = {
			serviceAccount = {
				workloadBindings = {
					pageInfo = {
						totalCount = 1,
					},
					nodes = {
						{
							id = State.binding5ID,
							workloadName = "isolated-app",
							serviceAccount = {
								id = State.sa2ID,
							},
						},
					},
				},
			},
		},
	}
end)

-- Test 14: Non-member cannot add binding
Test.gql("Non-member cannot add binding to team-beta SA", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query(string.format([[
		mutation {
			addWorkloadToServiceAccount(input: {
				serviceAccountID: "%s"
				environment: "dev"
				teamSlug: "team-beta"
				workloadName: "sneaky-app"
			}) {
				binding {
					id
				}
			}
		}
	]], State.sa2ID))

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

-- Test 15: Non-member cannot remove binding
Test.gql("Non-member cannot remove binding from team-beta SA", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query(string.format([[
		mutation {
			removeWorkloadFromServiceAccount(input: { bindingID: "%s" }) {
				bindingDeleted
			}
		}
	]], State.binding5ID))

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

-- Test 16: First SA still has its bindings unchanged
Test.gql("First SA bindings unchanged after second SA operations", function(t)
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
						totalCount = 3,
					},
				},
			},
		},
	}
end)
