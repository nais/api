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

Test.gql("Update service account as non-admin", function(t)
	t.query(string.format([[
		mutation {
			updateServiceAccount(
				input: {
					id: "%s"
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
					id: "%s"
					description: "new description"
				}
			) {
				serviceAccount {
					id
					description
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com"})

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


Test.gql("Add role to service account as non-admin", function(t)
	t.query(string.format([[
		mutation {
			addRoleToServiceAccount(
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
					"addRoleToServiceAccount",
				},
			},
		},
	}
end)

Test.gql("Add role to service account as admin", function(t)
	t.query(string.format([[
		mutation {
			addRoleToServiceAccount(
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
	]], State.saID), { ["x-user-email"] = "admin@example.com"})

	t.check {
		data = {
			addRoleToServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Team creator",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("delete service account as non-admin", function(t)
	t.query(string.format([[
		mutation {
			deleteServiceAccount(
				input: {
					id: "%s"
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


Test.gql("delete service account as admin", function(t)
	t.query(string.format([[
		mutation {
			deleteServiceAccount(
				input: {
					id: "%s"
				}
			) {
				serviceAccountDeleted
			}
		}
	]], State.saID), { ["x-user-email"] = "admin@example.com"})

	t.check {
    		data = {
    			deleteServiceAccount = {
    				serviceAccountDeleted = true,
    			}
    		},
    	}
end)
