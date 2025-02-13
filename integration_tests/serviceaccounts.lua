Test.gql("Create global service account as non-admin", function(t)
	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "my-sa"
					description: "some description"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"service_accounts:create\" authorization."),
				path = {
					"createServiceAccount",
				},
			},
		},
	}
end)

Helper.SQLExec [[
	INSERT INTO users (name, email, external_id, admin)
	VALUES ('Admin User', 'admin@example.com', '123', true)
]]

Test.gql("Create global service account as admin", function(t)
	t.query([[
		mutation {
			createServiceAccount(
				input: {
					name: "my-sa"
					description: "some description"
				}
			) {
				serviceAccount {
					id
					description
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("saID"),
					description = "some description",
					roles = {
						nodes = {},
					},
				},
			},
		},
	}
end)

Test.gql("Create team service account as admin", function(t)
	t.query([[
		mutation {
			createServiceAccount(
				input: {
					name: "team-sa"
					description: "some description"
					teamSlug: "slug-1"
				}
			) {
				serviceAccount {
					id
					description
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = NotNull(),
					description = "some description",
					roles = {
						nodes = {},
					},
				},
			},
		},
	}
end)

Test.gql("Update service account as non-admin", function(t)
	t.query(string.format([[
		mutation {
			updateServiceAccount(
				input: {
					serviceAccountID: "%s"
					description: "new description"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"service_accounts:update\" authorization."),
				path = {
					"updateServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Update service account as admin", function(t)
	t.query(string.format([[
		mutation {
			updateServiceAccount(
				input: {
					serviceAccountID: "%s"
					description: "new description"
				}
			) {
				serviceAccount {
					id
					description
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = {
			updateServiceAccount = {
				serviceAccount = {
					id = State.saID,
					description = "new description",
				},
			},
		},
	}
end)

Test.gql("Assign role to service account as non-admin", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"service_accounts:update\" authorization."),
				path = {
					"assignRoleToServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Assign role to service account as admin", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team owner"
				}
			) {
				serviceAccount {
					id
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Team owner",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Assign duplicate role to service account as admin", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team owner"
				}
			) {
				serviceAccount {
					id
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com" })

	t.check {
		errors = {
			{
				message = Contains("has already been assigned the \"Team owner\" role"),
				path = {
					"assignRoleToServiceAccount",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Assign another role to service account as admin", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Team creator",
							},
							{
								name = "Team owner",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Revoke role from service account as non-admin", function(t)
	t.query(string.format([[
		mutation {
			revokeRoleFromServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"service_accounts:update\" authorization."),
				path = {
					"revokeRoleFromServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Revoke role from service account as admin", function(t)
	t.query(string.format([[
		mutation {
			revokeRoleFromServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = {
			revokeRoleFromServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Team owner",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Revoke unassigned role from service account as admin", function(t)
	t.query(string.format([[
		mutation {
			revokeRoleFromServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Deploy key viewer"
				}
			) {
				serviceAccount {
					id
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("does not have the \"Deploy key viewer\" role assigned"),
				path = {
					"revokeRoleFromServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Delete service account as non-admin", function(t)
	t.query(string.format([[
		mutation {
			deleteServiceAccount(
				input: {
					serviceAccountID: "%s"
				}
			) {
				serviceAccountDeleted
			}
		}
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"service_accounts:delete\" authorization."),
				path = {
					"deleteServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Delete service account as admin", function(t)
	t.query(string.format([[
		mutation {
			deleteServiceAccount(
				input: {
					serviceAccountID: "%s"
				}
			) {
				serviceAccountDeleted
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com" })

	t.check {
		data = {
			deleteServiceAccount = {
				serviceAccountDeleted = true,
			},
		},
	}
end)
