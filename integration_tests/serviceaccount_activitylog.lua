-- Three accounts share the name "shared-name": one per team, plus a tenant-wide one. The activity log is
-- keyed by resource name, so these tests guard against one account's log leaking into another's.
local admin = User.new("admin", "admin@adminsen.com", "4332")
admin:admin(true)

Team.new("team-a", "purpose", "#channel")
Team.new("team-b", "purpose", "#channel")

Test.gql("Create tenant-wide service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: { name: "shared-name", description: "tenant wide" }
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
					id = Save("tenantSaID"),
				},
			},
		},
	}
end)

Test.gql("Create service account with the same name in team-a", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: { name: "shared-name", description: "team a", teamSlug: "team-a" }
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
					id = Save("teamASaID"),
				},
			},
		},
	}
end)

Test.gql("Create service account with the same name in team-b", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: { name: "shared-name", description: "team b", teamSlug: "team-b" }
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
					id = Save("teamBSaID"),
				},
			},
		},
	}
end)

-- A second entry, so the accounts differ by count as well as by team.
Test.gql("Update the team-a service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			updateServiceAccount(
				input: { serviceAccountID: "%s", description: "team a updated" }
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.teamASaID))

	t.check {
		data = {
			updateServiceAccount = {
				serviceAccount = {
					id = NotNull(),
				},
			},
		},
	}
end)

Test.gql("Team-scoped account only sees its own entries", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				activityLog(first: 10) {
					nodes {
						teamSlug
						resourceName
						resourceType
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]], State.teamASaID))

	t.check {
		data = {
			serviceAccount = {
				activityLog = {
					nodes = {
						{
							teamSlug = "team-a",
							resourceName = "shared-name",
							resourceType = "SERVICE_ACCOUNT",
						},
						{
							teamSlug = "team-a",
							resourceName = "shared-name",
							resourceType = "SERVICE_ACCOUNT",
						},
					},
					pageInfo = {
						totalCount = 2,
					},
				},
			},
		},
	}
end)

Test.gql("Other team's account with the same name is unaffected", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				activityLog(first: 10) {
					nodes {
						teamSlug
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]], State.teamBSaID))

	t.check {
		data = {
			serviceAccount = {
				activityLog = {
					nodes = {
						{
							teamSlug = "team-b",
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

-- The regression that motivated ListForResourceAndTeam. Without it, this returns all four entries.
Test.gql("Tenant-wide account does not see team-scoped entries", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				activityLog(first: 10) {
					nodes {
						teamSlug
					}
					pageInfo {
						totalCount
					}
					facets {
						activityTypes {
							activityType
							count
						}
					}
				}
			}
		}
	]], State.tenantSaID))

	t.check {
		data = {
			serviceAccount = {
				activityLog = {
					nodes = {
						{
							teamSlug = Null,
						},
					},
					pageInfo = {
						totalCount = 1,
					},
					facets = {
						activityTypes = {
							{
								activityType = "SERVICE_ACCOUNT_CREATED",
								count = 1,
							},
						},
					},
				},
			},
		},
	}
end)
