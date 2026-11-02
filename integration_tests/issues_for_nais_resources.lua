Helper.readK8sResources("k8s_resources/issues_nais")
local user = User.new()
Team.new("myteam", "purpose", "#slack_channel")
local checker = IssueChecker.new()
checker:runChecks()

for _, resource in ipairs {
	{ field = "openSearch", issueType = "OPENSEARCH", typename = "OpenSearchIssue", serviceType = "opensearch" },
	{ field = "valkey",     issueType = "VALKEY",     typename = "ValkeyIssue",     serviceType = "valkey" },
} do
	Test.gql(resource.typename .. " resolves nais.io resource", function(t)
		t.addHeader("x-user-email", user:email())
		t.query(string.format([[
			{
				team(slug: "myteam") {
					issues(filter: {issueType: %s, environments: ["dev-gcp"]}) {
						nodes {
							__typename
							message
							... on %s {
								%s {
									name
									issues {
										nodes {
											__typename
										}
									}
								}
							}
						}
					}
				}
			}
		]], resource.issueType, resource.typename, resource.field))
		t.check {
			data = {
				team = {
					issues = {
						nodes = {
							{
								__typename = resource.typename,
								message = "Your " .. resource.serviceType .. " service name reports: error message from aiven",
								[resource.field] = {
									name = "name",
									issues = {
										nodes = {
											{ __typename = resource.typename },
										},
									},
								},
							},
						},
					},
				},
			},
		}
	end)
end
