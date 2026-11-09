local owner = User.new("owner", "owner@example.com", "owner")
local otherOwner = User.new("other-owner", "other-owner@example.com", "other-owner")
local admin = User.new("admin", "admin@example.com", "admin")
admin:admin(true)

local ownerTeam = Team.new("team-owner", "Owner", "#owner")
local otherTeam = Team.new("team-other", "Other", "#other")
ownerTeam:addOwner(owner)
otherTeam:addOwner(otherOwner)

for _, tt in ipairs({
	{ name = "Own team by default",                    selection = "",                      teams = { "team-owner" } },
	{ name = "Null source teams defaults to own team", selection = "sourceTeamSlugs: null", teams = { "team-owner" } },
	{
		name = "Other teams without source-team permissions",
		selection = 'sourceTeamSlugs: ["team-other", "team-other"]',
		teams = { "team-other" },
	},
	{
		name = "Owner and other team",
		selection = 'sourceTeamSlugs: ["team-owner", "team-other"]',
		teams = { "team-owner", "team-other" },
	},
	{ name = "All teams without admin permissions", selection = "sourceTeamSlugs: []", teams = {} },
}) do
	Test.gql(tt.name, function(t)
		t.addHeader("x-user-email", owner:email())
		t.query(string.format([[
			mutation {
				createWebhook(input: {
					name: "  %s  "
					teamSlug: "team-owner"
					%s
					url: "https://webhook.example.invalid/webhook"
					secret: "test-secret"
					events: { allEvents: true }
				}) {
					webhook {
						id name teamSlug sourceTeamSlugs hasSecret
						deliveries(first: 10) { pageInfo { totalCount } }
					}
				}
			}
		]], tt.name, tt.selection))
		t.check {
			data = {
				createWebhook = {
					webhook = {
						id = Save("webhookID"),
						name = tt.name,
						teamSlug = "team-owner",
						sourceTeamSlugs = tt.teams,
						hasSecret = true,
						deliveries = { pageInfo = { totalCount = 0 } },
					},
				},
			},
		}
	end)
end

for _, tt in ipairs({
	{ name = "Reject an empty name on creation", mutation = "createWebhook", value = "" },
	{ name = "Reject a blank name on creation",  mutation = "createWebhook", value = "   " },
	{ name = "Reject an empty rename",           mutation = "updateWebhook", value = "" },
	{ name = "Reject a blank rename",            mutation = "updateWebhook", value = "   " },
}) do
	Test.gql(tt.name, function(t)
		t.addHeader("x-user-email", owner:email())
		local input
		if tt.mutation == "createWebhook" then
			input = string.format([[
				name: %q teamSlug: "team-owner" url: "https://webhook.example.invalid/webhook"
				secret: "test-secret" events: { allEvents: true }
			]], tt.value)
		else
			input = string.format('id: "%s" name: %q', State.webhookID, tt.value)
		end
		t.query(string.format([[
			mutation {
				%s(input: { %s }) { webhook { id } }
			}
		]], tt.mutation, input))
		t.check {
			data = Null,
			errors = {
				{
					locations = NotNull(),
					message = "Webhook name must not be empty.",
					path = { tt.mutation },
				},
			},
		}
	end)
end

for _, tt in ipairs({
	{ name = "Unrelated update preserves all teams",  selection = "enabled: false",                   teams = {} },
	{ name = "Null source teams preserves all teams", selection = "name: null sourceTeamSlugs: null", teams = {} },
	{
		name = "Switch all teams to explicit list",
		selection = 'sourceTeamSlugs: ["team-other"]',
		teams = { "team-other" },
	},
	{ name = "Unrelated update preserves source list",  selection = "enabled: true",         teams = { "team-other" } },
	{ name = "Null source teams preserves source list", selection = "sourceTeamSlugs: null", teams = { "team-other" } },
	{ name = "Switch list to all teams",                selection = "sourceTeamSlugs: []",   teams = {} },
	{
		name = "Switch back to an explicit team list",
		selection = 'sourceTeamSlugs: ["team-owner", "team-other"]',
		teams = { "team-owner", "team-other" },
	},
}) do
	Test.gql(tt.name, function(t)
		t.addHeader("x-user-email", owner:email())
		t.query(string.format([[
			mutation {
				updateWebhook(input: { id: "%s" %s }) {
					webhook { name teamSlug sourceTeamSlugs hasSecret }
				}
			}
		]], State.webhookID, tt.selection))
		t.check {
			data = {
				updateWebhook = {
					webhook = {
						name = "All teams without admin permissions",
						teamSlug = "team-owner",
						sourceTeamSlugs = tt.teams,
						hasSecret = true,
					},
				},
			},
		}
	end)
end

Test.gql("Nonexistent source team", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		mutation {
			updateWebhook(input: { id: "%s" sourceTeamSlugs: ["missing-team"] }) {
				webhook { id }
			}
		}
	]], State.webhookID))
	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = 'Source team "missing-team" does not exist.',
				path = { "updateWebhook" },
			},
		},
	}
end)

Test.gql("Source team cannot manage the owning team's webhook", function(t)
	t.addHeader("x-user-email", otherOwner:email())
	t.query(string.format([[
		mutation {
			updateWebhook(input: { id: "%s" sourceTeamSlugs: [] }) {
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
end)

Test.gql("All teams still requires authorization in the owner team", function(t)
	t.addHeader("x-user-email", otherOwner:email())
	t.query [[
		mutation {
			createWebhook(input: {
				name: "Test webhook"
				teamSlug: "team-owner"
				sourceTeamSlugs: []
				url: "https://webhook.example.invalid/webhook"
				secret: "test-secret"
				events: { allEvents: true }
			}) {
				webhook { id }
			}
		}
	]]
	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains('Specifically, you need the "webhooks:create" authorization.'),
				path = { "createWebhook" },
			},
		},
	}
end)

Test.gql("Legacy admin global webhooks remain global", function(t)
	t.addHeader("x-user-email", admin:email())
	t.query [[
		mutation {
			createWebhook(input: {
				name: "Test webhook"
				url: "https://webhook.example.invalid/webhook"
				secret: "test-secret"
				events: { allEvents: true }
			}) {
				webhook { teamSlug sourceTeamSlugs hasSecret }
			}
		}
	]]
	t.check {
		data = {
			createWebhook = {
				webhook = {
					teamSlug = Null,
					sourceTeamSlugs = {},
					hasSecret = true,
				},
			},
		},
	}
end)

Test.gql("Accept an eight-character signing secret", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query [[
		mutation {
			createWebhook(input: {
				name: "Test webhook"
				teamSlug: "team-owner"
				url: "https://webhook.example.invalid/secret-test"
				secret: "12345678"
				events: { allEvents: true }
			}) {
				webhook { id hasSecret }
			}
		}
	]]
	t.check {
		data = {
			createWebhook = {
				webhook = {
					id = Save("secretWebhookID"),
					hasSecret = true,
				},
			},
		},
	}
end)

for _, tt in ipairs({
	{ name = "Replace signing secret",                input = 'secret: "replacement-secret"' },
	{ name = "Omitted secret preserves stored value", input = "enabled: false" },
	{ name = "Null secret preserves stored value",    input = "secret: null" },
}) do
	Test.gql(tt.name, function(t)
		t.addHeader("x-user-email", owner:email())
		t.query(string.format([[
			mutation {
				updateWebhook(input: { id: "%s" %s }) {
					webhook { hasSecret }
				}
			}
		]], State.secretWebhookID, tt.input))
		t.check {
			data = {
				updateWebhook = {
					webhook = { hasSecret = true },
				},
			},
		}
		local row = Helper.SQLQueryRow([[
			SELECT secret = $1 AS matches
			FROM webhook_subscriptions
			WHERE url = 'https://webhook.example.invalid/secret-test'
		]], "replacement-secret")
		assert(row.matches, "Stored signing secret was not preserved or replaced correctly")
	end)
end

for _, tt in ipairs({
	{ name = "empty",                    value = "" },
	{ name = "seven ASCII characters",   value = "1234567" },
	{ name = "seven Unicode characters", value = string.rep("\195\184", 7) },
}) do
	for _, mutation in ipairs({ "createWebhook", "updateWebhook" }) do
		Test.gql(mutation .. " rejects a secret with " .. tt.name, function(t)
			t.addHeader("x-user-email", owner:email())
			local input
			if mutation == "createWebhook" then
				input = [=[
					name: "Rejected secret" teamSlug: "team-owner"
					url: "https://webhook.example.invalid/rejected-secret" events: { allEvents: true }
				]=]
			else
				input = string.format('id: "%s"', State.secretWebhookID)
			end
			t.query(string.format([[
				mutation {
					%s(input: { secret: "%s" %s }) { webhook { id } }
				}
			]], mutation, tt.value, input))
			t.check {
				data = Null,
				errors = {
					{
						locations = NotNull(),
						message = "Webhook secret must be at least 8 characters.",
						path = { mutation },
					},
				},
			}
			local row = Helper.SQLQueryRow([[
				SELECT secret = 'replacement-secret' AS matches
				FROM webhook_subscriptions
				WHERE url = 'https://webhook.example.invalid/secret-test'
			]])
			assert(row.matches, "Rejected secrets must not replace the stored secret")
		end)
	end
end

for _, tt in ipairs({
	{ team = "team-owner", user = owner,      key = "sharedWebhookID" },
	{ team = "team-other", user = otherOwner, key = "otherSharedWebhookID" },
}) do
	Test.gql("Create a shared webhook name in " .. tt.team, function(t)
		t.addHeader("x-user-email", tt.user:email())
		t.query(string.format([[
			mutation {
				createWebhook(input: {
					name: "Shared webhook"
					teamSlug: "%s"
					sourceTeamSlugs: ["team-other"]
					url: "https://webhook.example.invalid/shared"
					secret: "test-secret"
					events: { allEvents: true }
				}) {
					webhook { id name teamSlug }
				}
			}
		]], tt.team))
		t.check {
			data = {
				createWebhook = {
					webhook = {
						id = Save(tt.key),
						name = "Shared webhook",
						teamSlug = tt.team,
					},
				},
			},
		}
	end)
end

for _, mutation in ipairs({ "createWebhook", "updateWebhook" }) do
	Test.gql(mutation .. " rejects a duplicate name in the owning team", function(t)
		t.addHeader("x-user-email", owner:email())
		local before = Helper.SQLQueryRow(
			"SELECT COUNT(*) AS count FROM activity_log_entries WHERE resource_type = 'WEBHOOK'")
		local input
		if mutation == "createWebhook" then
			input = [=[
				teamSlug: "team-owner" sourceTeamSlugs: [] url: "https://webhook.example.invalid/duplicate"
				secret: "test-secret" events: { allEvents: true }
			]=]
		else
			input = string.format('id: "%s"', State.webhookID)
		end
		t.query(string.format([[
			mutation {
				%s(input: { name: "  Shared webhook  " %s }) { webhook { id } }
			}
		]], mutation, input))
		t.check {
			data = Null,
			errors = {
				{
					locations = NotNull(),
					message = "A webhook with this name already exists in the team.",
					path = { mutation },
				},
			},
		}
		local after = Helper.SQLQueryRow(
			"SELECT COUNT(*) AS count FROM activity_log_entries WHERE resource_type = 'WEBHOOK'")
		assert(tonumber(before.count) == tonumber(after.count), "Rejected duplicates must not create audit entries")
		local row = Helper.SQLQueryRow([[
			SELECT COUNT(*) AS count FROM webhook_subscriptions
			WHERE team_slug = 'team-owner' AND name = 'Shared webhook'
		]])
		assert(tonumber(row.count) == 1, "The owning team must retain exactly one webhook with this name")
		t.query(string.format([[
			query {
				node(id: "%s") { ... on WebhookSubscription { name } }
			}
		]], State.webhookID))
		t.check {
			data = { node = { name = "All teams without admin permissions" } },
		}
	end)
end

Test.gql("Updating a webhook to its own trimmed name is allowed", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		mutation {
			updateWebhook(input: { id: "%s" name: "  Shared webhook  " }) {
				webhook { id name }
			}
		}
	]], State.sharedWebhookID))
	t.check {
		data = {
			updateWebhook = {
				webhook = { id = State.sharedWebhookID, name = "Shared webhook" },
			},
		},
	}
end)

Test.gql("Testing a webhook requires permission in the owning team", function(t)
	t.addHeader("x-user-email", otherOwner:email())
	t.query(string.format([[
		mutation { testWebhook(input: { id: "%s" }) { delivery { id } } }
	]], State.secretWebhookID))
	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains('webhooks:update'),
				path = { "testWebhook" },
			},
		},
	}
end)

Test.gql("Manual tests record failures without retries or changes to subscription health", function(t)
	t.addHeader("x-user-email", owner:email())
	local before = Helper.SQLQueryRow(
		"SELECT COUNT(*) AS count FROM activity_log_entries WHERE resource_type = 'WEBHOOK'")
	Helper.SQLExec([[
		UPDATE webhook_subscriptions SET enabled = FALSE, consecutive_failures = 9
		WHERE url = 'https://webhook.example.invalid/secret-test'
	]])
	t.query(string.format([[
		mutation {
			testWebhook(input: { id: "%s" }) {
				delivery {
					id eventType isTest success errorMessage
					requestBody requestHeaders { name values }
					responseBody responseBodyTruncated
					responseStatus responseHeaders { name values }
					deliveryState { status nextRetryAt attemptCount }
				}
			}
		}
	]], State.secretWebhookID))
	t.check {
		data = {
			testWebhook = {
				delivery = {
					id = Save("testDeliveryID"),
					eventType = "WEBHOOK_TEST",
					isTest = true,
					success = false,
					errorMessage = NotNull(),
					requestBody = NotNull(),
					requestHeaders = NotNull(),
					responseBody = Null,
					responseBodyTruncated = false,
					responseStatus = Null,
					responseHeaders = {},
					deliveryState = { status = "FAILED", nextRetryAt = Null, attemptCount = 1 },
				},
			},
		},
	}
	local row = Helper.SQLQueryRow([[
		SELECT NOT enabled AND consecutive_failures = 9 AS unchanged,
			(SELECT COUNT(*) FROM webhook_deliveries WHERE subscription_id = webhook_subscriptions.id) AS attempts,
			(SELECT COUNT(*) FROM webhook_event_deliveries WHERE subscription_id = webhook_subscriptions.id) AS queued
		FROM webhook_subscriptions WHERE url = 'https://webhook.example.invalid/secret-test'
	]])
	assert(row.unchanged, "Manual tests must not affect subscription health")
	assert(tonumber(row.attempts) == 1 and tonumber(row.queued) == 0,
		"Manual tests must make exactly one attempt without a retry")
	local after = Helper.SQLQueryRow(
		"SELECT COUNT(*) AS count FROM activity_log_entries WHERE resource_type = 'WEBHOOK'")
	assert(tonumber(before.count) == tonumber(after.count), "Manual tests must not create activity log entries")
end)

Test.gql("Manual webhook tests are rate limited", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query(string.format([[
		mutation { testWebhook(input: { id: "%s" }) { delivery { id } } }
	]], State.secretWebhookID))
	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "Wait at least 30 seconds between webhook tests.",
				path = { "testWebhook" },
			},
		},
	}
end)

Test.gql("Delivery diagnostics preserve each attempt's outcome and retry snapshot", function(t)
	t.addHeader("x-user-email", owner:email())
	Helper.SQLExec([[
		INSERT INTO webhook_event_deliveries (webhook_event_id, subscription_id, retry_count, run_at)
		SELECT event.id, subscription.id, 2, NOW() + INTERVAL '1 minute'
		FROM webhook_subscriptions subscription
		JOIN activity_log_entries activity
			ON activity.resource_type = 'WEBHOOK' AND activity.resource_name = subscription.id::text AND activity.action = 'CREATED'
		JOIN webhook_events event ON event.activity_log_entries_id = activity.id
		WHERE subscription.team_slug = 'team-owner' AND subscription.name = 'Shared webhook'
	]])
	Helper.SQLExec([[
		INSERT INTO webhook_deliveries (
			subscription_id, webhook_event_delivery_id, event_type, request_body,
			response_status, response_body, duration_ms, success, attempt_count, next_retry_at, created_at
		)
		SELECT delivery.subscription_id, delivery.id, 'TEAM_UPDATED', CONVERT_TO('{"test":true}', 'UTF8'),
			CASE WHEN attempt.n = 1 THEN 200 ELSE 500 END,
			DECODE('ff0001', 'hex'), 1, attempt.n = 1, 3 - attempt.n,
			CASE WHEN attempt.n = 2 THEN NOW() - INTERVAL '1 minute' END,
			NOW() - attempt.n * INTERVAL '1 second'
		FROM webhook_event_deliveries delivery
		JOIN webhook_subscriptions subscription ON subscription.id = delivery.subscription_id
		CROSS JOIN (VALUES (1), (2)) attempt(n)
		WHERE subscription.team_slug = 'team-owner' AND subscription.name = 'Shared webhook'
	]])
	for _, phase in ipairs({
		{ enabled = true,  status = "pending" },
		{ enabled = false, status = "processing" },
		{ enabled = true,  status = "completed" },
		{ enabled = false, status = "failed" },
	}) do
		Helper.SQLExec(string.format([[
			UPDATE webhook_subscriptions SET enabled = %s
			WHERE team_slug = 'team-owner' AND name = 'Shared webhook'
		]], phase.enabled and "TRUE" or "FALSE"))
		Helper.SQLExec(string.format([[
			UPDATE webhook_event_deliveries SET status = '%s', retry_count = 7, run_at = NOW()
			WHERE subscription_id IN (
				SELECT id FROM webhook_subscriptions WHERE team_slug = 'team-owner' AND name = 'Shared webhook'
			)
		]], phase.status))
		t.query(string.format([[
			query {
				node(id: "%s") {
					... on WebhookSubscription {
						deliveries(first: 10) {
							nodes {
								id requestBody responseBody isTest
								deliveryState { status attemptCount nextRetryAt }
							}
						}
					}
				}
			}
		]], State.sharedWebhookID))
		local expected = {
			{
				id = Save("successfulAttemptID"),
				requestBody = '{"test":true}',
				responseBody = Null,
				isTest = false,
				deliveryState = { status = "COMPLETED", attemptCount = 2, nextRetryAt = Null },
			},
			{
				id = Save("failedAttemptID"),
				requestBody = '{"test":true}',
				responseBody = Null,
				isTest = false,
				deliveryState = { status = "FAILED", attemptCount = 1, nextRetryAt = NotNull() },
			},
		}
		t.check { data = { node = { deliveries = { nodes = expected } } } }
		t.query(string.format([[
			query {
				node(id: "%s") {
					... on WebhookDelivery {
						deliveryState { status attemptCount nextRetryAt }
					}
				}
			}
		]], State.failedAttemptID))
		t.check {
			data = {
				node = {
					deliveryState = { status = "FAILED", attemptCount = 1, nextRetryAt = NotNull() },
				},
			},
		}
	end
	Helper.SQLExec([[
		DELETE FROM webhook_event_deliveries
		WHERE subscription_id IN (
			SELECT id FROM webhook_subscriptions WHERE team_slug = 'team-owner' AND name = 'Shared webhook'
		)
	]])
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on WebhookDelivery {
					responseBody deliveryState { status attemptCount nextRetryAt } isTest
				}
			}
		}
	]], State.failedAttemptID))
	t.check {
		data = {
			node = {
				responseBody = Null,
				deliveryState = { status = "FAILED", attemptCount = 1, nextRetryAt = NotNull() },
				isTest = false,
			},
		},
	}
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on WebhookSubscription {
					deliveries(first: 10) {
						nodes { deliveryState { status attemptCount nextRetryAt } }
						pageInfo { totalCount }
					}
				}
			}
		}
	]], State.sharedWebhookID))
	t.check {
		data = {
			node = {
				deliveries = {
					nodes = {
						{ deliveryState = { status = "COMPLETED", attemptCount = 2, nextRetryAt = Null } },
						{ deliveryState = { status = "FAILED", attemptCount = 1, nextRetryAt = NotNull() } },
					},
					pageInfo = { totalCount = 2 },
				},
			},
		},
	}
end)

Test.gql("Create an all-events subscription without an event list", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query [[
		mutation {
			createWebhook(input: {
				name: "Event selection", teamSlug: "team-owner",
				url: "https://hooks.example.com", secret: "test-secret", events: { allEvents: true }
			}) { webhook { id allEvents eventTypes } }
		}
	]]
	t.check {
		data = { createWebhook = { webhook = {
			id = Save("eventSelectionWebhookID"), allEvents = true, eventTypes = {},
		} } },
	}
end)

for _, tt in ipairs({
	{ name = "Explicit events replace all events",         input = 'events: { eventTypes: ["TEAM_UPDATED"] }', allEvents = false, eventTypes = { "TEAM_UPDATED" } },
	{ name = "Unrelated updates preserve explicit events", input = 'name: "Event selection renamed"',          allEvents = false, eventTypes = { "TEAM_UPDATED" } },
	{ name = "All events clears the explicit list",        input = "events: { allEvents: true }",              allEvents = true,  eventTypes = {} },
	{ name = "Unrelated updates preserve all events",      input = "enabled: false",                           allEvents = true,  eventTypes = {} },
	{ name = "Null events preserves selection",            input = "events: null",                             allEvents = true,  eventTypes = {} },
}) do
	Test.gql(tt.name, function(t)
		t.addHeader("x-user-email", owner:email())
		t.query(string.format([[
			mutation {
				updateWebhook(input: { id: "%s", %s }) { webhook { allEvents eventTypes } }
			}
		]], State.eventSelectionWebhookID, tt.input))
		t.check { data = { updateWebhook = { webhook = {
			allEvents = tt.allEvents, eventTypes = tt.eventTypes,
		} } } }
	end)
end

for _, tt in ipairs({
	{ name = "False allEvents", input = "events: { allEvents: false }",  message = "allEvents must be true" },
	{ name = "Empty list",      input = "events: { eventTypes: [] }",    message = "Select at least one event type" },
	{ name = "Legacy wildcard", input = 'events: { eventTypes: ["*"] }', message = "invalid event type" },
}) do
	for _, mutation in ipairs({ "createWebhook", "updateWebhook" }) do
		Test.gql(mutation .. " rejects " .. tt.name, function(t)
			t.addHeader("x-user-email", owner:email())
			local input = string.format('id: "%s", %s', State.eventSelectionWebhookID, tt.input)
			if mutation == "createWebhook" then
				input =
					'name: "Invalid selection", teamSlug: "team-owner", url: "https://hooks.example.com", secret: "test-secret", ' ..
					tt.input
			end

			t.query(string.format("mutation { %s(input: { %s }) { webhook { id } } }", mutation, input))
			t.check {
				data = Null,
				errors = { { message = Contains(tt.message), locations = NotNull(), path = { mutation } } },
			}
		end)
	end
end

for _, tt in ipairs({
	{ name = "Both fields",                 selection = '{ allEvents: true, eventTypes: ["TEAM_UPDATED"] }', message = "must specify exactly one key" },
	{ name = "Both fields with empty list", selection = "{ allEvents: true, eventTypes: [] }",               message = "must specify exactly one key" },
	{ name = "Both fields with null",       selection = "{ allEvents: true, eventTypes: null }",             message = "must specify exactly one key" },
	{ name = "Neither field",               selection = "{}",                                                message = "must specify exactly one key" },
	{ name = "Null field",                  selection = "{ allEvents: null }",                               message = "must be non-null" },
}) do
	for _, mutation in ipairs({ "createWebhook", "updateWebhook" }) do
		Test.gql(mutation .. " enforces oneOf for " .. tt.name, function(t)
			t.addHeader("x-user-email", owner:email())
			local input = string.format('id: "%s", events: %s', State.eventSelectionWebhookID, tt.selection)
			if mutation == "createWebhook" then
				input =
					'name: "Invalid oneOf", teamSlug: "team-owner", url: "https://hooks.example.com", secret: "test-secret", events: ' ..
					tt.selection
			end
			t.query(string.format("mutation { %s(input: { %s }) { webhook { id } } }", mutation, input))
			t.check {
				data = Null,
				errors = { { message = Contains(tt.message), locations = NotNull(), extensions = Ignore() } },
			}
		end)
	end
end
