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
local one_hour_ago = now - 60 * 60
local thirty_one_days_ago = now - 31 * 24 * 60 * 60
local thirty_days_ago = now - 30 * 24 * 60 * 60

local from = iso8601_with_tz(ten_minutes_ago)
local to = iso8601_with_tz(five_minutes_ago)
local one_hour_ago_str = iso8601_with_tz(one_hour_ago)
local now_str = iso8601_with_tz(now)
local thirty_one_days_ago_str = iso8601_with_tz(thirty_one_days_ago)
local thirty_days_ago_str = iso8601_with_tz(thirty_days_ago)

-- Instant query tests

Test.gql("Metrics - instant query for specific environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		query {
		  environment(name: "dev") {
				metrics(input: {query: "up"}) {
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
		}
	]])

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

Test.gql("Metrics - instant query with specific time", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up"
					time: "%s"
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
		}
	]],
		from
	))

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

Test.gql("Metrics - custom PromQL instant query", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "kube_pod_container_resource_requests{resource=\"cpu\"}"
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
		}
	]])

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

-- Range query tests

Test.gql("Metrics - range query with valid parameters", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "rate(container_cpu_usage_seconds_total[5m])"
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
		}
	]],
		from,
		to
	))

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

Test.gql("Metrics - range query with minimum step (10 seconds)", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up"
					range: {start: "%s", end: "%s", step: 10}
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
		}
	]],
		from,
		to
	))

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

Test.gql("Metrics - range query with maximum allowed time range (30 days)", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up"
					range: {start: "%s", end: "%s", step: 300}
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
		}
	]],
		thirty_days_ago_str,
		now_str
	))

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

-- Validation error tests

Test.gql("Metrics - error on step less than 10 seconds", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up"
					range: {start: "%s", end: "%s", step: 5}
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
		}
	]],
		from,
		to
	))

	t.check {
		data = Null,
		errors = {
			{
				message = "Query step size must be at least 10 seconds. Please increase the step size to reduce the number of data points.",
				path = { "environment", "metrics" },
			},
		},
	}
end)

Test.gql("Metrics - error on time range exceeding 30 days", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up"
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
		}
	]],
		thirty_one_days_ago_str,
		now_str
	))

	t.check {
		data = Null,
		errors = {
			{
				message = "The time range is too large. Maximum allowed is 30 days, but you requested 744h0m0s. Please reduce the time range.",
				path = { "environment", "metrics" },
			},
		},
	}
end)

Test.gql("Metrics - error on exceeding maximum data points (11000)", function(t)
	t.addHeader("x-user-email", user:email())
	-- 1 hour range with 10 second step = 360 data points (valid)
	-- But 30 days with 10 second step would exceed limit
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up"
					range: {start: "%s", end: "%s", step: 10}
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
		}
	]],
		thirty_days_ago_str,
		now_str
	))

	t.check {
		data = Null,
		errors = {
			{
				message = "This query would return too many data points (259201). The maximum allowed is 11000. Please increase the step size or reduce the time range.",
				path = { "environment", "metrics" },
			},
		},
	}
end)

Test.gql("Metrics - error on end time before start time", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up"
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
		}
	]],
		to,
		from
	))

	t.check {
		data = Null,
		errors = {
			{
				message = "The end time must be after the start time. Please check your time range.",
				path = { "environment", "metrics" },
			},
		},
	}
end)

Test.gql("Metrics - error on empty query", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		query {
		  environment(name: "dev") {
				metrics(input: {query: ""}) {
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
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = "Failed to query metrics: unknown position: parse error: no expression found in input",
				path = { "environment", "metrics" },
			},
		},
	}
end)

-- Complex PromQL query tests

Test.gql("Metrics - complex aggregation query", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "sum(rate(http_requests_total[5m])) by (job)"
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
		}
	]])

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

Test.gql("Metrics - query with label filters", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "up{job=\"prometheus\",instance=~\".*:9090\"}"
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
		}
	]])

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)

Test.gql("Metrics - range query with rate function", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[
		query {
		  environment(name: "dev") {
				metrics(input: {
					query: "rate(process_cpu_seconds_total[1m])"
					range: {start: "%s", end: "%s", step: 30}
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
		}
	]],
		from,
		to
	))

	t.check {
		data = {
			environment = {
				metrics = {
					series = NotNull(),
					warnings = {},
				},
			},
		},
	}
end)
