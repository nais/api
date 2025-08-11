Helper.readK8sResources("k8s_resources/simple")

local user = User.new("user-1", "usr@ex.com", "ei")
Team.new("slug-1", "team-name", "#team")

local function iso8601_with_tz(ts)
	local s = os.date("%Y-%m-%dT%H:%M:%S%z", ts)
	-- s is a string at runtime, but assert to satisfy the type checker if needed:
	assert(type(s) == "string")
	return s:sub(1, -3) .. ":" .. s:sub(-2)
end

local now          = os.time()
local one_week_ago = now - 7 * 24 * 60 * 60

local from         = iso8601_with_tz(one_week_ago)
local to           = iso8601_with_tz(now)


Test.gql("Ingress metrics", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		query {
			team(slug: "slug-1") {
				applications {
					nodes {
						name
						ingresses {
							metrics {
								requestsPerSecond
								errorsPerSecond
								requestsPerSecondSeries: series(input: {type: REQUESTS_PER_SECOND, start: "%s", end: "%s"}) {
									timestamp
									value
								}
							}

						}
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]], from, to))

	t.check {
		data = {
			team = {
				applications = {
					nodes = {
						{
							name = "another-app",
							ingresses = {
								{
									metrics = {
										requestsPerSecond = NotNull(),
										errorsPerSecond = NotNull(),
										requestsPerSecondSeries = NotNull(),
									},
								},
							},
						},
						{
							name = "app-name",
							ingresses = {
								{
									metrics = {
										requestsPerSecond = NotNull(),
										errorsPerSecond = NotNull(),
										requestsPerSecondSeries = NotNull(),
									},
								},
							},
						},
					},
					pageInfo = {
						totalCount = 2,
					},
				},
			},
		},
	}
end)
