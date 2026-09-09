Helper.readK8sResources("k8s_resources/kafka_acl_filter")

local team = Team.new("devteam", "purpose", "#slack-channel")
local user = User.new()

Test.gql("topic without filter", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
		  team(slug: "devteam") {
		    environment(name: "dev") {
		      kafkaTopic(name: "dokument") {
		        acl {
		          nodes {
		            workloadName
		            teamName
		            access
		          }
		        }
		      }
		    }
		  }
		}
	]])

	t.check {
		data = {
			team = {
				environment = {
					kafkaTopic = {
						acl = {
							nodes = {
								{
									access = "READWRITE",
									teamName = "*",
									workloadName = "all",
								},
								{
									access = "READ",
									teamName = "devteam",
									workloadName = "*",
								},
								{
									access = "READWRITE",
									teamName = "devteam",
									workloadName = "app1",
								},
								{
									access = "READWRITE",
									teamName = "devteam",
									workloadName = "missing",
								},
								{
									access = "READWRITE",
									teamName = "otherteam",
									workloadName = "app2",
								},
								{
									access = "READWRITE",
									teamName = "otherteam",
									workloadName = "jobname-1",
								},
								{
									access = "READWRITE",
									teamName = "otherteam",
									workloadName = "missing",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("topic filtering for workload", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
		  team(slug: "devteam") {
		    environment(name: "dev") {
		      kafkaTopic(name: "dokument") {
		        acl(filter: { workload: "app1" }) {
		          nodes {
		            workloadName
		            teamName
		            access
		          }
		        }
		      }
		    }
		  }
		}
	]])

	t.check {
		data = {
			team = {
				environment = {
					kafkaTopic = {
						acl = {
							nodes = {
								{ workloadName = "*",    teamName = "devteam", access = "READ" },
								{ workloadName = "app1", teamName = "devteam", access = "READWRITE" },
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("topic filtering for team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
		  team(slug: "devteam") {
		    environment(name: "dev") {
		      kafkaTopic(name: "dokument") {
		        acl(filter: { team: "otherteam" }) {
		          nodes {
		            workloadName
		            teamName
		            access
		          }
		        }
		      }
		    }
		  }
		}
	]])

	t.check {
		data = {
			team = {
				environment = {
					kafkaTopic = {
						acl = {
							nodes = {
								{ workloadName = "all",       teamName = "*",         access = "READWRITE" },
								{ workloadName = "app2",      teamName = "otherteam", access = "READWRITE" },
								{ workloadName = "jobname-1", teamName = "otherteam", access = "READWRITE" },
								{ workloadName = "missing",   teamName = "otherteam", access = "READWRITE" },
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("topic filtering for valid workloads", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
		  team(slug: "devteam") {
		    environment(name: "dev") {
		      kafkaTopic(name: "dokument") {
		        acl(filter: { validWorkloads: true }) {
		          nodes {
		            workloadName
		            teamName
		            access
		          }
		        }
		      }
		    }
		  }
		}
	]])

	t.check {
		data = {
			team = {
				environment = {
					kafkaTopic = {
						acl = {
							nodes = {
								{ workloadName = "all",       teamName = "*",         access = "READWRITE" },
								{ workloadName = "*",         teamName = "devteam",   access = "READ" },
								{ workloadName = "app1",      teamName = "devteam",   access = "READWRITE" },
								{ workloadName = "app2",      teamName = "otherteam", access = "READWRITE" },
								{ workloadName = "jobname-1", teamName = "otherteam", access = "READWRITE" },
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("topic filtering for invalid workloads", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
		  team(slug: "devteam") {
		    environment(name: "dev") {
		      kafkaTopic(name: "dokument") {
		        acl(filter: { validWorkloads: false }) {
		          nodes {
		            workloadName
		            teamName
		            access
		          }
		        }
		      }
		    }
		  }
		}
	]])

	t.check {
		data = {
			team = {
				environment = {
					kafkaTopic = {
						acl = {
							nodes = {
								{ workloadName = "missing", teamName = "devteam",   access = "READWRITE" },
								{ workloadName = "missing", teamName = "otherteam", access = "READWRITE" },
							},
						},
					},
				},
			},
		},
	}
end)
