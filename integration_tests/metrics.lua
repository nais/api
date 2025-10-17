local user = User.new("user-1", "usr@ex.com", "ei")
Team.new("slug-1", "team-name", "#team")

local function iso8601_with_tz(ts)
	local s = os.date("%Y-%m-%dT%H:%M:%S%z", ts)
	assert(type(s) == "string")
	return s:sub(1, -3) .. ":" .. s:sub(-2)
end

local now = os.time()
local five_minutes_ago = now - 5 * 60
local ten_minutes_ago = now - 10 * 60

local from = iso8601_with_tz(ten_minutes_ago)
local to = iso8601_with_tz(five_minutes_ago)

Test.gql("Metrics - instant query for specific environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		query {
			metrics(input: {query: "up", environmentName: "dev"}) {
				series {
					labels {
						name
						value
					}
					values {
						timestamp
						value
					}
				}
				warnings
			}
		}
	]])

	t.check {
		data = {
			metrics = {
				series = NotNull(),
				warnings = Null,
			},
		},
	}
end)

Test.gql("Metrics - range query", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
			metrics(input: {
				query: "rate(container_cpu_usage_seconds_total[5m])"
				environmentName: "dev"
				range: {start: "%s", end: "%s", step: 60}
			}) {
				series {
					labels {
						name
						value
					}
					values {
						timestamp
						value
					}
				}
				warnings
			}
		}
	]],
		from,
		to
	))

	t.check {
		data = {
			metrics = {
				series = NotNull(),
				warnings = Null,
			},
		},
	}
end)

Test.gql("Metrics - custom PromQL query", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		query {
			metrics(input: {
				query: "kube_pod_container_resource_requests{resource=\"cpu\"}"
				environmentName: "dev"
			}) {
				series {
					labels {
						name
						value
					}
					values {
						timestamp
						value
					}
				}
				warnings
			}
		}
	]])

	t.check {
		data = {
			metrics = {
				series = NotNull(),
				warnings = Null,
			},
		},
	}
end)
