Helper.SQLExec([[
	DELETE FROM user_roles WHERE role_name = 'Team owner';
]])

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

Helper.SQLExec [[
	INSERT INTO users (name, email, external_id)
	VALUES ('Non Member', 'non-member@example.com', '123')
]]

Helper.SQLExec([[
	INSERT INTO
		user_roles (role_name, user_id, target_team_slug)
	VALUES (
		'Team member',
		(SELECT id FROM users WHERE email = 'non-member@example.com'),
		'slug-2'
	)
	ON CONFLICT DO NOTHING;
	;
]])

Test.gql("Create service account as non-member", function(t)
	t.query([[
		mutation {
			createServiceAccount(
				input: {
					name: "my-sa"
					description: "some description"
					teamSlug: "slug-1"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], { ["x-user-email"] = "non-member@example.com" })

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

Test.gql("Create service account as member", function(t)
	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "my-sa"
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
	]]

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

Test.gql("Update service account as non-member", function(t)
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
	]], State.saID), { ["x-user-email"] = "non-member@example.com" })

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

Test.gql("Update service account as member", function(t)
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
	]], State.saID))

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

Test.gql("Assign role to service account as non-member", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team member"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "non-member@example.com" })

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

Test.gql("Assign global role to service account as member", function(t)
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
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = "Role \"Team creator\" is only allowed on global service accounts.",
				path = {
					"assignRoleToServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Assign role to service account as member", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
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
	]], State.saID))

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Deploy key viewer",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Assign duplicate role to service account as member", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
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
	]], State.saID))

	t.check {
		errors = {
			{
				message = Contains("has already been assigned the \"Deploy key viewer\" role"),
				path = {
					"assignRoleToServiceAccount",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Assign another role to service account as member", function(t)
	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team member"
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
	]], State.saID))

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Deploy key viewer",
							},
							{
								name = "Team member",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Assign another role to service account as member that requires owner", function(t)
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
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = "User does not have permission to assign the \"Team owner\" role.",
				path = {
					"assignRoleToServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Revoke role from service account as non-member", function(t)
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
	]], State.saID), { ["x-user-email"] = "non-member@example.com" })

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

Test.gql("Revoke role from service account as member", function(t)
	t.query(string.format([[
		mutation {
			revokeRoleFromServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team member"
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
	]], State.saID))

	t.check {
		data = {
			revokeRoleFromServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Deploy key viewer",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Revoke unassigned role from service account as member", function(t)
	t.query(string.format([[
		mutation {
			revokeRoleFromServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Service account owner"
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
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = "Service account does not have the \"Service account owner\" role assigned.",
				path = {
					"revokeRoleFromServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Revoke role from service account as member that requires owner", function(t)
	t.query(string.format([[
		mutation {
			revokeRoleFromServiceAccount(
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
	]], State.saID))

	t.check {
		data = Null,
		errors = {
			{
				message = "User does not have permission to revoke the \"Team owner\" role.",
				path = {
					"revokeRoleFromServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Delete service account as non-member", function(t)
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
	]], State.saID), { ["x-user-email"] = "non-member@example.com" })

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

Test.gql("Delete service account as member", function(t)
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
		data = {
			deleteServiceAccount = {
				serviceAccountDeleted = true,
			},
		},
	}
end)
