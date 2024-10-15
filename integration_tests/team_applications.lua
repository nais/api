Helper.readK8sResources("k8s_resources/simple")

Test.gql("Team with multiple applications", function(t)
	t.query [[
		query {
			team(slug: "slug-1") {
				applications {
					nodes {
						name
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				applications = {
					nodes = {
						{
							name = "another-app"
						},
						{
							name = "app-name"
						}
					},
					pageInfo = {
						totalCount = 2
					}
				}
			}
		}
	}
end)

Test.gql("Team with one application", function(t)
	t.query [[
    		query {
    			team(slug: "slug-2") {
    				applications {
    					nodes {
    						name
    					}
    					pageInfo {
    						totalCount
    					}
    				}
    			}
    		}
    	]]

	t.check {
		data = {
			team = {
				applications = {
					nodes = {
						{
							name = "app-name"
						}
					},
					pageInfo = {
						totalCount = 1
					}
				}
			}
		}
	}
end)

Test.gql("Team with no applications", function(t)
	t.query [[
    		query {
    			team(slug: "slug-3") {
    				applications {
    					pageInfo {
    						totalCount
    					}
    				}
    			}
    		}
    	]]

	t.check {
		data = {
			team = {
				applications = {
					pageInfo = {
						totalCount = 0
					}
				}
			}
		}
	}
end)
