local reconcilers = { "reconciler-1", "reconciler-2" }

for _, reconciler in ipairs(reconcilers) do
	Helper.SQLExec("INSERT INTO reconcilers (name, display_name, description) VALUES ($1, $1, '')", reconciler)

	local configs = {
		{ name = "config-secret",  secret = true },
		{ name = "config-visible", secret = false },
	}

	for _, config in ipairs(configs) do
		Helper.SQLExec([[
				INSERT INTO reconciler_config (reconciler, key, display_name, description, secret)
				VALUES ($1, $2, $2 || '_display', $2 || '_description', $3)
			]], reconciler, config.name, config.secret)
	end
end

Test.gql("list reconcilers as non-admin", function(t)
	t.query [[
		query {
			reconcilers {
				nodes {
					name
					enabled
					configured
				}
				pageInfo {
					totalCount
				}
			}
		}
	]]

	t.check {
		data = {
			reconcilers = {
				nodes = {
					{ name = "reconciler-1", enabled = false, configured = false },
					{ name = "reconciler-2", enabled = false, configured = false },
				},
				pageInfo = {
					totalCount = 2,
				},
			},
		},
	}
end)


Test.gql("list reconcilers with config as non-admin", function(t)
	t.query [[
		query {
			reconcilers {
				nodes {
					name
					enabled
					configured
					config {
						key
						displayName
						description
						configured
						secret
						value
					}
				}
				pageInfo {
					totalCount
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = "You are authenticated, but your account is not authorized to perform this action.",
				path = { "reconcilers", "nodes", Ignore(), "config" },
			},
			{
				message = "You are authenticated, but your account is not authorized to perform this action.",
				path = { "reconcilers", "nodes", Ignore(), "config" },
			},
		},
	}
end)

Test.gql("enable reconciler as non-admin", function(t)
	t.query [[
		mutation {
			enableReconciler(input: { name: "reconciler-1" }) {
				name
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = "You are authenticated, but your account is not authorized to perform this action.",
				path = { "enableReconciler" },
			},
		},
	}
end)

Test.gql("disable reconciler as non-admin", function(t)
	t.query [[
		mutation {
			disableReconciler(input: { name: "reconciler-1" }) {
				name
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = "You are authenticated, but your account is not authorized to perform this action.",
				path = { "disableReconciler" },
			},
		},
	}
end)

-- Make the authenticated user an admin
Helper.SQLExec("UPDATE users SET admin = true WHERE email = $1",
	'authenticated@example.com')

Test.gql("enable non-configured reconciler as admin", function(t)
	t.query [[
		mutation {
			enableReconciler(input: { name: "reconciler-1" }) {
				name
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = "Reconciler is not fully configured, missing one or more options: config-secret, config-visible",
				path = { "enableReconciler" },
			},
		},
	}
end)

local valuesToSet = {
	{ key = "config-secret",  value = "secret" },
	{ key = "config-visible", value = "visible" },
}

for _, value in ipairs(valuesToSet) do
	Test.gql(string.format("configure %s for reconciler as admin", value.key), function(t)
		t.query(string.format([[
		mutation {
			configureReconciler(input: { name: "reconciler-1", config: {key: "%s", value: "%s"} }) {
				name
			}
		}
	]], value.key, value.value))

		t.check {
			data = {
				configureReconciler = {
					name = "reconciler-1",
				},
			},
		}
	end)
end

Test.gql("enable configured reconciler as admin", function(t)
	t.query [[
		mutation {
			enableReconciler(input: { name: "reconciler-1" }) {
				name
				enabled
				configured
				config {
					key
					displayName
					description
					configured
					secret
					value
				}
			}
		}
	]]

	t.check {
		data = {
			enableReconciler = {
				name = "reconciler-1",
				enabled = true,
				configured = true,
				config = {
					{
						key = "config-secret",
						displayName = "config-secret_display",
						description = "config-secret_description",
						configured = true,
						secret = true,
						value = Null,
					},
					{
						key = "config-visible",
						displayName = "config-visible_display",
						description = "config-visible_description",
						configured = true,
						secret = false,
						value = "visible",
					},
				},
			},
		},
	}
end)


Test.gql("disable reconciler as admin", function(t)
	t.query [[
		mutation {
			disableReconciler(input: { name: "reconciler-2" }) {
				name
			}
		}
	]]

	t.check {
		data = {
			disableReconciler = {
				name = "reconciler-2",
			},
		},
	}
end)

Test.gql("list reconcilers after modifications", function(t)
	t.query [[
		query {
			reconcilers {
				nodes {
					activityLog {
						nodes {
							message
							... on ReconcilerConfiguredActivityLogEntry {
								data {
									updatedKeys
								}
							}
						}
					}
					name
					enabled
				}
				pageInfo {
					totalCount
				}
			}
		}
	]]

	t.check {
		data = {
			reconcilers = {
				nodes = {
					{
						activityLog = {
							nodes = {
								{ message = "Enable reconciler" },
								{
									message = "Configure reconciler",
									data = {
										updatedKeys = { "config-visible" },
									},
								},
								{
									message = "Configure reconciler",
									data = {
										updatedKeys = { "config-secret" },
									},
								},
							},
						},
						name = "reconciler-1",
						enabled = true,
					},
					{
						activityLog = {
							nodes = {
								{ message = "Disable reconciler" },
							},
						},
						name = "reconciler-2",
						enabled = false,
					},
				},
				pageInfo = {
					totalCount = 2,
				},
			},
		},
	}
end)
