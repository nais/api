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
								{ workloadName = "*",       teamName = "devteam",   access = "read" },
								{ workloadName = "all",     teamName = "*",         access = "readwrite" },
								{ workloadName = "app1",    teamName = "devteam",   access = "readwrite" },
								{ workloadName = "app2",    teamName = "otherteam", access = "readwrite" },
								{ workloadName = "missing", teamName = "devteam",   access = "readwrite" },
								{ workloadName = "missing", teamName = "otherteam", access = "readwrite" },
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
								{ workloadName = "*",    teamName = "devteam", access = "read" },
								{ workloadName = "app1", teamName = "devteam", access = "readwrite" },
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
								{ workloadName = "all",     teamName = "*",         access = "readwrite" },
								{ workloadName = "app2",    teamName = "otherteam", access = "readwrite" },
								{ workloadName = "missing", teamName = "otherteam", access = "readwrite" },
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
								{ workloadName = "*",    teamName = "devteam",   access = "read" },
								{ workloadName = "all",  teamName = "*",         access = "readwrite" },
								{ workloadName = "app1", teamName = "devteam",   access = "readwrite" },
								{ workloadName = "app2", teamName = "otherteam", access = "readwrite" },
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
								{ workloadName = "missing", teamName = "devteam",   access = "readwrite" },
								{ workloadName = "missing", teamName = "otherteam", access = "readwrite" },
							},
						},
					},
				},
			},
		},
	}
end)
