local owner = User.new("owner", "owner@example.com", "owner")
local outsider = User.new("outsider", "outsider@example.com", "outsider")
local admin = User.new("admin", "admin@example.com", "admin")
admin:admin(true)
local team = Team.new("team-owner", "Owner", "#owner")
team:addOwner(owner)

local function checkTeamEntry(t, activityType, entry)
	t.query(string.format([[
		query {
			team(slug: "team-owner") {
				activityLog(first: 10, filter: { resourceTypes: [WEBHOOK], activityTypes: [%s] }) {
					nodes {
						__typename actor message resourceType resourceName teamSlug
						... on WebhookUpdatedActivityLogEntry {
							data { updatedFields { field oldValue newValue } }
						}
					}
					pageInfo { totalCount }
				}
			}
		}
	]], activityType))
	entry.actor = owner:email()
	entry.resourceType = "WEBHOOK"
	entry.teamSlug = "team-owner"
	t.check {
		data = {
			team = {
				activityLog = {
					nodes = { entry },
					pageInfo = { totalCount = 1 },
				},
			},
		},
	}
end

Test.gql("Creating a webhook records the actor, owner and subscription identifier", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query [[
		mutation {
			createWebhook(input: {
				teamSlug: "team-owner"
				url: "http://127.0.0.1:1/old-url-token"
				secret: "old-signing-secret"
				eventTypes: ["TEAM_UPDATED"]
			}) {
				webhook { id }
			}
		}
	]]
	t.check {
		data = {
			createWebhook = {
				webhook = { id = Save("webhookID") },
			},
		},
	}
	checkTeamEntry(t, "WEBHOOK_CREATED", {
		__typename = "WebhookCreatedActivityLogEntry",
		message = "Created webhook",
		resourceName = Save("webhookResourceName"),
	})
	local row = Helper.SQLQueryRow([[
		SELECT resource_name = subscription.id::text AS matches
		FROM activity_log_entries, webhook_subscriptions subscription
		WHERE resource_type = 'WEBHOOK' AND action = 'CREATED'
	]])
	assert(row.matches, "Webhook activity must identify the subscription, not its endpoint URL")
end)

Test.gql("Webhook updates record changes without storing signing secrets or endpoint URLs", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		mutation {
			updateWebhook(input: {
				id: "%s"
				url: "http://127.0.0.1:1/new-url-token"
				secret: "new-signing-secret"
				eventTypes: ["TEAM_MEMBER_ADDED"]
				sourceTeamSlugs: []
				enabled: false
			}) {
				webhook { id }
			}
		}
	]], State.webhookID))
	t.check {
		data = {
			updateWebhook = {
				webhook = { id = State.webhookID },
			},
		},
	}
	checkTeamEntry(t, "WEBHOOK_UPDATED", {
		__typename = "WebhookUpdatedActivityLogEntry",
		message = "Updated webhook",
		resourceName = State.webhookResourceName,
		data = {
			updatedFields = {
				{ field = "url",             oldValue = Null,             newValue = Null },
				{ field = "secret",          oldValue = Null,             newValue = Null },
				{ field = "eventTypes",      oldValue = "[TEAM_UPDATED]", newValue = "[TEAM_MEMBER_ADDED]" },
				{ field = "sourceTeamSlugs", oldValue = "[team-owner]",   newValue = "[]" },
				{ field = "enabled",         oldValue = "true",           newValue = "false" },
			},
		},
	})
	local row = Helper.SQLQueryRow([[
		SELECT COALESCE(BOOL_AND(
			CONVERT_FROM(data, 'UTF8') NOT LIKE '%signing-secret%'
			AND CONVERT_FROM(data, 'UTF8') NOT LIKE '%url-token%'
		), TRUE) AS redacted
		FROM activity_log_entries
		WHERE resource_type = 'WEBHOOK'
	]])
	assert(row.redacted, "Webhook credentials must not be persisted in activity log data")
end)

Test.gql("Unauthorized webhook mutations do not record successful activity", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query(string.format([[
		mutation {
			updateWebhook(input: { id: "%s" enabled: true }) {
				webhook { id }
			}
		}
	]], State.webhookID))
	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains('Specifically, you need the "webhooks:update" authorization.'),
				path = { "updateWebhook" },
			},
		},
	}
	local row = Helper.SQLQueryRow("SELECT COUNT(*) AS count FROM activity_log_entries WHERE resource_type = 'WEBHOOK'")
	assert(tonumber(row.count) == 2, "Unauthorized mutation must not create an activity log entry")
end)

for _, tt in ipairs({
	{
		name = "Create",
		mutation = "createWebhook",
		input = [=[teamSlug: "team-owner" url: "http://127.0.0.1:1/failed" secret: "secret" eventTypes: ["*"]]=],
		fields = "webhook { id }",
	},
	{ name = "Update", mutation = "updateWebhook", input = "enabled: true", fields = "webhook { id }" },
	{ name = "Delete", mutation = "deleteWebhook", input = "",              fields = "webhookID" },
}) do
	Test.gql(tt.name .. " rolls back when activity logging fails", function(t)
		t.addHeader("x-user-email", owner:email())
		Helper.SQLExec([[
			ALTER TABLE activity_log_entries
			ADD CONSTRAINT reject_webhook_test_activity CHECK (resource_type != 'WEBHOOK') NOT VALID
		]])
		local input = tt.input
		if tt.mutation ~= "createWebhook" then
			input = string.format('id: "%s" %s', State.webhookID, input)
		end
		t.query(string.format([[
			mutation {
				%s(input: { %s }) { %s }
			}
		]], tt.mutation, input, tt.fields))
		t.check {
			data = Null,
			errors = {
				{
					locations = NotNull(),
					message = NotNull(),
					path = { tt.mutation },
				},
			},
		}
		Helper.SQLExec("ALTER TABLE activity_log_entries DROP CONSTRAINT reject_webhook_test_activity")
		local row = Helper.SQLQueryRow([[
			SELECT COUNT(*) AS count, BOOL_AND(NOT enabled) AS disabled
			FROM webhook_subscriptions
		]])
		assert(tonumber(row.count) == 1 and row.disabled, "Failed audit must roll back the webhook mutation")
	end)
end

Test.gql("Deleting a webhook preserves its activity log", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		mutation {
			deleteWebhook(input: { id: "%s" }) { webhookID }
		}
	]], State.webhookID))
	t.check {
		data = {
			deleteWebhook = { webhookID = State.webhookID },
		},
	}
	checkTeamEntry(t, "WEBHOOK_DELETED", {
		__typename = "WebhookDeletedActivityLogEntry",
		message = "Deleted webhook",
		resourceName = State.webhookResourceName,
	})
	local row = Helper.SQLQueryRow("SELECT COUNT(*) AS count FROM activity_log_entries WHERE resource_type = 'WEBHOOK'")
	assert(tonumber(row.count) == 3, "Deleting a webhook must preserve all three activity log entries")
end)

Test.gql("Global webhook activity has no owning team", function(t)
	t.addHeader("x-user-email", admin:email())
	t.query [[
		mutation {
			createWebhook(input: {
				url: "http://127.0.0.1:1/global"
				secret: "global-secret"
				eventTypes: ["TEAM_UPDATED"]
			}) {
				webhook { id }
			}
		}
	]]
	t.check {
		data = {
			createWebhook = {
				webhook = { id = Save("globalWebhookID") },
			},
		},
	}
	t.query [[
		query {
			activityLog(first: 10, filter: { activityTypes: [WEBHOOK_CREATED] }) {
				nodes { __typename actor teamSlug }
				pageInfo { totalCount }
			}
		}
	]]
	t.check {
		data = {
			activityLog = {
				nodes = {
					{ __typename = "WebhookCreatedActivityLogEntry", actor = admin:email(), teamSlug = Null },
					{ __typename = "WebhookCreatedActivityLogEntry", actor = owner:email(), teamSlug = "team-owner" },
				},
				pageInfo = { totalCount = 2 },
			},
		},
	}
end)

for _, tt in ipairs({
	{ name = "Primary", key = "primaryWebhookID" },
	{ name = "Sibling", key = "siblingWebhookID" },
}) do
	Test.gql(tt.name .. " webhook can share an endpoint with other subscriptions", function(t)
		t.addHeader("x-user-email", owner:email())
		t.query [[
			mutation {
				createWebhook(input: {
					teamSlug: "team-owner"
					url: "http://127.0.0.1:1/global"
					secret: "secret"
					eventTypes: ["TEAM_UPDATED"]
				}) {
					webhook { id }
				}
			}
		]]
		t.check {
			data = {
				createWebhook = {
					webhook = { id = Save(tt.key) },
				},
			},
		}
	end)
end

Test.gql("Add a second activity entry to only the primary subscription", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		mutation {
			updateWebhook(input: { id: "%s" enabled: false }) {
				webhook { id }
			}
		}
	]], State.primaryWebhookID))
	t.check {
		data = {
			updateWebhook = {
				webhook = { id = State.primaryWebhookID },
			},
		},
	}
end)

Test.gql("Webhook ActivityLogger scopes entries and facets to the subscription ID", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on ActivityLogger {
					activityLog(first: 1) {
						nodes { id __typename resourceName }
						pageInfo { totalCount hasNextPage hasPreviousPage endCursor }
						facets { resourceTypes { resourceType count } }
					}
				}
			}
		}
	]], State.primaryWebhookID))
	t.check {
		data = {
			node = {
				activityLog = {
					nodes = {
						{
							id = Save("primaryUpdatedEntryID"),
							__typename = "WebhookUpdatedActivityLogEntry",
							resourceName = Save("primaryResourceName"),
						},
					},
					pageInfo = {
						totalCount = 2,
						hasNextPage = true,
						hasPreviousPage = false,
						endCursor = Save("primaryAfterCursor"),
					},
					facets = {
						resourceTypes = { { resourceType = "WEBHOOK", count = 2 } },
					},
				},
			},
		},
	}
end)

Test.gql("Subscription activity supports forward pagination", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on WebhookSubscription {
					activityLog(first: 1, after: "%s") {
						nodes { __typename resourceName }
						pageInfo { totalCount hasNextPage hasPreviousPage endCursor }
					}
				}
			}
		}
	]], State.primaryWebhookID, State.primaryAfterCursor))
	t.check {
		data = {
			node = {
				activityLog = {
					nodes = {
						{ __typename = "WebhookCreatedActivityLogEntry", resourceName = State.primaryResourceName },
					},
					pageInfo = {
						totalCount = 2,
						hasNextPage = false,
						hasPreviousPage = true,
						endCursor = Save("primaryCreatedCursor"),
					},
				},
			},
		},
	}
end)

Test.gql("Subscription activity supports backward pagination", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on WebhookSubscription {
					activityLog(last: 1, before: "%s") {
						nodes { id }
						pageInfo { totalCount hasNextPage hasPreviousPage }
					}
				}
			}
		}
	]], State.primaryWebhookID, State.primaryCreatedCursor))
	t.check {
		data = {
			node = {
				activityLog = {
					nodes = { { id = State.primaryUpdatedEntryID } },
					pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false },
				},
			},
		},
	}
end)

for _, key in ipairs({ "primaryWebhookID", "siblingWebhookID", "globalWebhookID" }) do
	Test.gql(key .. " supports filters without including other subscriptions", function(t)
		t.addHeader("x-user-email", admin:email())
		t.query(string.format([[
			query {
				node(id: "%s") {
					... on WebhookSubscription {
						activityLog(first: 10, filter: { activityTypes: [WEBHOOK_CREATED], resourceTypes: [WEBHOOK] }) {
							nodes { __typename }
							pageInfo { totalCount }
							facets { resourceTypes { resourceType count } }
						}
					}
				}
			}
		]], State[key]))
		t.check {
			data = {
				node = {
					activityLog = {
						nodes = { { __typename = "WebhookCreatedActivityLogEntry" } },
						pageInfo = { totalCount = 1 },
						facets = {
							resourceTypes = { { resourceType = "WEBHOOK", count = 1 } },
						},
					},
				},
			},
		}
	end)
end
