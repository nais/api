local owner = User.new("owner", "owner@example.com", "owner")
local member = User.new("member", "member@example.com", "member")
local outsider = User.new("outsider", "outsider@example.com", "outsider")
local admin = User.new("admin", "admin@example.com", "admin")
admin:admin(true)
local team = Team.new("team-owner", "Owner", "#owner")
team:addOwner(owner)
team:addMember(member)

local function checkTeamEntry(t, activityType, entry)
	t.query(string.format([[
		query {
			team(slug: "team-owner") {
				activityLog(first: 10, filter: { resourceTypes: [WEBHOOK], activityTypes: [%s] }) {
					nodes {
						__typename actor message resourceType resourceName teamSlug
						... on WebhookCreatedActivityLogEntry { data { name } }
						... on WebhookUpdatedActivityLogEntry {
							data { name updatedFields { field oldValue newValue } }
						}
						... on WebhookDeletedActivityLogEntry { data { name } }
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
				name: "Test webhook"
				teamSlug: "team-owner"
				url: "https://webhook.example.invalid/old-url-token"
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
		message = 'Created webhook "Test webhook"',
		resourceName = Save("webhookResourceName"),
		data = { name = "Test webhook" },
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
				name: "Renamed webhook"
				url: "https://webhook.example.invalid/new-url-token"
				secret: "new-signing-secret"
				eventTypes: ["TEAM_MEMBER_ADDED"]
				sourceTeamSlugs: []
				enabled: false
			}) {
				webhook { id name }
			}
		}
	]], State.webhookID))
	t.check {
		data = {
			updateWebhook = {
				webhook = { id = State.webhookID, name = "Renamed webhook" },
			},
		},
	}
	checkTeamEntry(t, "WEBHOOK_UPDATED", {
		__typename = "WebhookUpdatedActivityLogEntry",
		message = 'Updated webhook "Renamed webhook"',
		resourceName = State.webhookResourceName,
		data = {
			name = "Renamed webhook",
			updatedFields = {
				{ field = "name",            oldValue = "Test webhook",   newValue = "Renamed webhook" },
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

Test.gql("Renaming keeps the UUID-scoped audit history and original name snapshot", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on WebhookSubscription {
					id name
					activityLog(first: 10) {
						nodes {
							resourceName message
							... on WebhookCreatedActivityLogEntry { data { name } }
							... on WebhookUpdatedActivityLogEntry { data { name } }
						}
						pageInfo { totalCount }
					}
				}
			}
		}
	]], State.webhookID))
	t.check {
		data = {
			node = {
				id = State.webhookID,
				name = "Renamed webhook",
				activityLog = {
					nodes = {
						{
							resourceName = State.webhookResourceName,
							message = 'Updated webhook "Renamed webhook"',
							data = { name = "Renamed webhook" },
						},
						{
							resourceName = State.webhookResourceName,
							message = 'Created webhook "Test webhook"',
							data = { name = "Test webhook" },
						},
					},
					pageInfo = { totalCount = 2 },
				},
			},
		},
	}
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
		input = [=[name: "Test webhook" teamSlug: "team-owner" url: "https://webhook.example.invalid/failed" secret: "test-secret" eventTypes: ["*"]]=],
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
		message = 'Deleted webhook "Renamed webhook"',
		resourceName = State.webhookResourceName,
		data = { name = "Renamed webhook" },
	})
	checkTeamEntry(t, "WEBHOOK_CREATED", {
		__typename = "WebhookCreatedActivityLogEntry",
		message = 'Created webhook "Test webhook"',
		resourceName = State.webhookResourceName,
		data = { name = "Test webhook" },
	})
	local row = Helper.SQLQueryRow("SELECT COUNT(*) AS count FROM activity_log_entries WHERE resource_type = 'WEBHOOK'")
	assert(tonumber(row.count) == 3, "Deleting a webhook must preserve all three activity log entries")
end)

Test.gql("Global webhook activity has no owning team", function(t)
	t.addHeader("x-user-email", admin:email())
	t.query [[
		mutation {
			createWebhook(input: {
				name: "Test webhook"
				url: "https://webhook.example.invalid/global"
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
		t.query(string.format([[
			mutation {
				createWebhook(input: {
					name: %q
					teamSlug: "team-owner"
					url: "https://webhook.example.invalid/global"
					secret: "test-secret"
					eventTypes: ["TEAM_UPDATED"]
				}) {
					webhook { id }
				}
			}
		]], tt.name))
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

Test.gql("Prepare delivery node authorization fixtures", function(t)
	t.addHeader("x-user-email", admin:email())
	Helper.SQLExec([[
		INSERT INTO webhook_deliveries (subscription_id, event_type, request_body, duration_ms, success)
		SELECT id, 'WEBHOOK_TEST', '{}'::bytea, 1, FALSE FROM webhook_subscriptions
	]])
	for _, key in ipairs({ "primaryWebhookID", "globalWebhookID" }) do
		t.query(string.format([[
			query {
				node(id: "%s") {
					... on WebhookSubscription { deliveries(first: 10) { nodes { id } } }
				}
			}
		]], State[key]))
		t.check {
			data = { node = { deliveries = { nodes = { { id = Save(key .. "DeliveryID") } } } } },
		}
	end
end)

Test.gql("Team members can read webhook lists, subscription activity and delivery nodes", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(string.format([[
		query {
			team(slug: "team-owner") { webhooks(first: 10) { pageInfo { totalCount } } }
			subscription: node(id: "%s") {
				... on WebhookSubscription {
					id url
					activityLog(first: 1) { pageInfo { totalCount } }
					deliveries(first: 10) { nodes { id } }
				}
			}
			delivery: node(id: "%s") {
				... on WebhookDelivery { id requestBody responseBody }
			}
		}
	]], State.primaryWebhookID, State.primaryWebhookIDDeliveryID))
	t.check {
		data = {
			team = { webhooks = { pageInfo = { totalCount = 2 } } },
			subscription = {
				id = State.primaryWebhookID,
				url = NotNull(),
				activityLog = { pageInfo = { totalCount = 2 } },
				deliveries = { nodes = { { id = State.primaryWebhookIDDeliveryID } } },
			},
			delivery = { id = State.primaryWebhookIDDeliveryID, requestBody = "{}", responseBody = Null },
		},
	}
end)

for _, key in ipairs({ "globalWebhookID", "globalWebhookIDDeliveryID" }) do
	Test.gql("Team members cannot read global webhook node " .. key, function(t)
		t.addHeader("x-user-email", member:email())
		t.query(string.format('query { node(id: "%s") { id } }', State[key]))
		t.check {
			data = { node = Null },
			errors = { { message = NotNull(), locations = NotNull(), path = { "node" } } },
		}
	end)
end

for _, tt in ipairs({
	{ mutation = "createWebhook", permission = "create" },
	{ mutation = "updateWebhook", permission = "update" },
	{ mutation = "deleteWebhook", permission = "delete" },
	{ mutation = "testWebhook",   permission = "update" },
}) do
	Test.gql("Team membership does not grant " .. tt.mutation, function(t)
		t.addHeader("x-user-email", member:email())
		local input = string.format('id: "%s"', State.primaryWebhookID)
		local selection = "webhook { id }"
		if tt.mutation == "createWebhook" then
			input =
			[=[name: "Member webhook", teamSlug: "team-owner", url: "https://hooks.example.com", secret: "test-secret", eventTypes: ["*"]]=]
		elseif tt.mutation == "updateWebhook" then
			input = input .. ", enabled: true"
		elseif tt.mutation == "deleteWebhook" then
			selection = "webhookID"
		elseif tt.mutation == "testWebhook" then
			selection = "delivery { id }"
		end
		t.query(string.format("mutation { %s(input: { %s }) { %s } }", tt.mutation, input, selection))
		t.check {
			data = Null,
			errors = {
				{
					message = Contains('"webhooks:' .. tt.permission .. '"'),
					locations = NotNull(),
					path = { tt.mutation },
				},
			},
		}
	end)
end

for _, key in ipairs({ "primaryWebhookID", "primaryWebhookIDDeliveryID", "globalWebhookID", "globalWebhookIDDeliveryID" }) do
	Test.gql("Outsiders cannot read webhook node " .. key, function(t)
		t.addHeader("x-user-email", outsider:email())
		t.query(string.format([[
			query {
				node(id: "%s") {
					... on WebhookSubscription { url activityLog(first: 1) { nodes { id } } }
					... on WebhookDelivery { requestBody responseBody requestHeaders { name values } }
				}
			}
		]], State[key]))
		t.check {
			data = { node = Null },
			errors = { { message = NotNull(), locations = NotNull(), path = { "node" } } },
		}
	end)
end

Test.gql("Outsiders cannot enumerate a team's webhook diagnostics", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query [[
		query { team(slug: "team-owner") { webhooks(first: 10) { nodes { url } } } }
	]]
	t.check {
		data = Null,
		errors = { { message = NotNull(), locations = NotNull(), path = { "team", "webhooks" } } },
	}
end)

Test.gql("Audit updates use the locked database row instead of a cached before-image", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query [[
		mutation {
			createWebhook(input: {
				name: "Audit before"
				teamSlug: "team-owner"
				url: "https://hooks.example.com/audit"
				secret: "test-secret"
				eventTypes: ["TEAM_UPDATED"]
			}) { webhook { id } }
		}
	]]
	t.check { data = { createWebhook = { webhook = { id = Save("auditWebhookID") } } } }
	t.query(string.format([[
		mutation {
			rename: updateWebhook(input: { id: "%s", name: "Audit after" }) { webhook { id } }
			disable: updateWebhook(input: { id: "%s", enabled: false }) { webhook { id } }
		}
	]], State.auditWebhookID, State.auditWebhookID))
	t.check {
		data = {
			rename = { webhook = { id = State.auditWebhookID } },
			disable = { webhook = { id = State.auditWebhookID } },
		},
	}
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on WebhookSubscription {
					activityLog(first: 1, filter: { activityTypes: [WEBHOOK_UPDATED] }) {
						nodes { ... on WebhookUpdatedActivityLogEntry { data { updatedFields { field oldValue newValue } } } }
					}
				}
			}
		}
	]], State.auditWebhookID))
	t.check {
		data = {
			node = {
				activityLog = {
					nodes = { { data = { updatedFields = { { field = "enabled", oldValue = "true", newValue = "false" } } } } },
				},
			},
		},
	}
end)

for _, url in ipairs({ "http://hooks.example.com", "https://127.0.0.1", "https://user:password@hooks.example.com" }) do
	for _, mutation in ipairs({ "createWebhook", "updateWebhook" }) do
		Test.gql(mutation .. " rejects unsafe URL " .. url, function(t)
			t.addHeader("x-user-email", owner:email())
			local input = string.format('id: "%s", url: %q', State.primaryWebhookID, url)
			if mutation == "createWebhook" then
				input = string.format(
					'name: "Unsafe URL", teamSlug: "team-owner", url: %q, secret: "test-secret", eventTypes: ["*"]', url)
			end
			t.query(string.format("mutation { %s(input: { %s }) { webhook { id } } }", mutation, input))
			t.check {
				data = Null,
				errors = { { message = NotNull(), locations = NotNull(), path = { mutation } } },
			}
		end)
	end
end

Test.gql("Tenant internal ingress URLs can be registered", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query [[
		mutation {
			createWebhook(input: {
				name: "Internal ingress", teamSlug: "team-owner",
				url: "https://myapp.intern.dev-nais.cloud.nais.io/events",
				secret: "test-secret", eventTypes: ["*"]
			}) { webhook { url } }
		}
	]]
	t.check {
		data = {
			createWebhook = { webhook = { url = "https://myapp.intern.dev-nais.cloud.nais.io/events" } },
		},
	}
end)
