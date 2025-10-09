Helper.readK8sResources("k8s_resources/activitylog_filter")

local user = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("Create valkey as team member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation CreateValkey {
		  createValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "%s"
		      tier: SINGLE_NODE
		      memory: GB_14
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]], team:slug()))

	t.check {
		data = {
			createValkey = {
				valkey = {
					name = "foobar",
				},
			},
		},
	}
end)

Test.gql("Delete app", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
			mutation {
				deleteApplication(
					input: {teamSlug: "%s", environmentName: "dev", name: "app"}
				) {
					success
				}
			}
		]], team:slug()))

	t.check {
		data = {
			deleteApplication = {
				success = true,
			},
		},
	}
end)

Test.gql("Delete valkey", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
			mutation {
				deleteValkey(
					input: {teamSlug: "%s", environmentName: "dev", name: "foobar"}
				) {
					valkeyDeleted
				}
			}
		]], team:slug()))

	t.check {
		data = {
			deleteValkey = {
				valkeyDeleted = true,
			},
		},
	}
end)

Test.gql("Query activitylog list", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query ActivtyLog {
		  team(slug: "%s") {
		    activityLog(
		      first: 20
		      filter: {
		        activityTypes: [
		          APPLICATION_DELETED
		          VALKEY_DELETED
		        ]
		      }
		    ) {
		      nodes {
		        __typename
		        message
		        actor
		        createdAt
		        resourceType
		        resourceName
		      }
		    }
		  }
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "ValkeyDeletedActivityLogEntry",
							message = "Deleted Valkey",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "VALKEY",
							resourceName = "foobar",
						},
						{
							__typename = "ApplicationDeletedActivityLogEntry",
							message = "Application deleted",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "APP",
							resourceName = "app",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Query activitylog list without valkey", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query ActivtyLog {
		  team(slug: "%s") {
		    activityLog(
		      first: 20
		      filter: {
		        activityTypes: [
		          APPLICATION_DELETED
		          APPLICATION_RESTARTED
		          APPLICATION_SCALED
		          DEPLOYMENT
		          JOB_DELETED
		          JOB_TRIGGERED
		          OPENSEARCH_CREATED
		          OPENSEARCH_UPDATED
		          OPENSEARCH_DELETED
		          OPENSEARCH_MAINTENANCE_STARTED
		          REPOSITORY_ADDED
		          REPOSITORY_REMOVED
		          SECRET_CREATED
		          SECRET_DELETED
		          SECRET_VALUE_ADDED
		          SECRET_VALUE_UPDATED
		          SECRET_VALUE_REMOVED
		          VALKEY_UPDATED
		          VALKEY_MAINTENANCE_STARTED
		        ]
		      }
		    ) {
		      nodes {
		        __typename
		        message
		        actor
		        createdAt
		        resourceType
		        resourceName
		      }
		    }
		  }
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "ApplicationDeletedActivityLogEntry",
							message = "Application deleted",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "APP",
							resourceName = "app",
						},
					},
				},
			},
		},
	}
end)
