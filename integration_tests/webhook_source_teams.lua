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
					teamSlug: "team-owner"
					%s
					url: "http://127.0.0.1:1/webhook"
					secret: "test-secret"
					eventTypes: ["*"]
				}) {
					webhook { id teamSlug sourceTeamSlugs hasSecret }
				}
			}
		]], tt.selection))
		t.check {
			data = {
				createWebhook = {
					webhook = {
						id = Save("webhookID"),
						teamSlug = "team-owner",
						sourceTeamSlugs = tt.teams,
						hasSecret = true,
					},
				},
			},
		}
	end)
end

for _, tt in ipairs({
	{ name = "Unrelated update preserves all teams",  selection = "enabled: false",        teams = {} },
	{ name = "Null source teams preserves all teams", selection = "sourceTeamSlugs: null", teams = {} },
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
					webhook { teamSlug sourceTeamSlugs hasSecret }
				}
			}
		]], State.webhookID, tt.selection))
		t.check {
			data = {
				updateWebhook = {
					webhook = {
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
				teamSlug: "team-owner"
				sourceTeamSlugs: []
				url: "http://127.0.0.1:1/webhook"
				secret: "test-secret"
				eventTypes: ["*"]
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
				url: "http://127.0.0.1:1/webhook"
				secret: "test-secret"
				eventTypes: ["*"]
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

Test.gql("Report when no signing secret is set", function(t)
	t.addHeader("x-user-email", owner:email())
	t.query [[
		mutation {
			createWebhook(input: {
				teamSlug: "team-owner"
				url: "http://127.0.0.1:1/secret-test"
				secret: ""
				eventTypes: ["*"]
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
					hasSecret = false,
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
			WHERE url = 'http://127.0.0.1:1/secret-test'
		]], "replacement-secret")
		assert(row.matches, "Stored signing secret was not preserved or replaced correctly")
	end)
end
