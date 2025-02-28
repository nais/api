local user = User.new("user", "user@user.com", "ext")
Team.new("slug-1", "team-1", "#team")
Team.new("slug-2", "team-1", "#team")

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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
user:admin(true)

Test.gql("enable non-configured reconciler as admin", function(t)
	t.addHeader("x-user-email", user:email())

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
		t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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

Test.gql("list reconciler errors", function(t)
	t.addHeader("x-user-email", user:email())

	Helper.SQLExec [[
		INSERT INTO reconciler_errors (correlation_id, reconciler, created_at, error_message, team_slug)
		VALUES
		(gen_random_uuid(), 'reconciler-1', CLOCK_TIMESTAMP(), 'first error for reconciler-1', 'slug-1'),
		(gen_random_uuid(), 'reconciler-1', CLOCK_TIMESTAMP(), 'second error for reconciler-1', 'slug-2'),
		(gen_random_uuid(), 'reconciler-2', CLOCK_TIMESTAMP(), 'will not be displayed because reconciler is disabled', 'slug-1')
	]]

	t.query [[
		query {
			reconcilers {
				nodes {
					name
					enabled
					errors {
						nodes {
							id
							message
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			reconcilers = {
				nodes = {
					{
						name = "reconciler-1",
						enabled = true,
						errors = {
							nodes = {
								{
									id = Contains("RECE_"),
									message = "second error for reconciler-1",
								},
								{
									id = Contains("RECE_"),
									message = "first error for reconciler-1",
								},
							},
						},
					},
					{
						name = "reconciler-2",
						enabled = false,
						errors = {
							nodes = {},
						},
					},
				},
			},
		},
	}
end)
