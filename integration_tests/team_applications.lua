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
							name = "another-app",
						},
						{
							name = "app-name",
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
							name = "app-name",
						},
					},
					pageInfo = {
						totalCount = 1,
					},
				},
			},
		},
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
						totalCount = 0,
					},
				},
			},
		},
	}
end)

Test.gql("Team with multiple applications and instances", function(t)
	t.query [[
		query {
			team(slug: "slug-1") {
				applications {
					nodes {
						name
						instances {
							nodes {
								name
								id
								restarts
								created
								status {
									state
									message
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
	]]

	t.check {
		data = {
			team = {
				applications = {
					nodes = {
						{
							name = "another-app",
							instances = {
								nodes = {
									{
										created = "2022-07-06T11:45:18+02:00",
										id = "INS_3reJsVY19Sss8MvJH7ghHt6WcbXHW4AqMH7QPYi5yvJr6qZLw936mCi2ZcD9eM",
										name = "another-app-23422-2sdf",
										restarts = 0,
										status = {
											message = "Unknown",
											state = "UNKNOWN",
										},
									},
								},
							},
						},
						{
							name = "app-name",
							instances = {
								nodes = {
									{
										created = "2022-07-06T11:45:18+02:00",
										id = "INS_2JN3xdYkjgBWnSYwiqQiRZV44TR5uGVaoEkRVTyLhY53YfVFGju1k9",
										name = "app-name-23422-2sdf",
										restarts = 0,
										status = {
											message = "Running",
											state = "RUNNING",
										},
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