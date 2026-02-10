local user = User.new("user", "user@usersen.com")
local nonMemberUser = User.new("nonmember", "other@user.com")

local mainTeam = Team.new("someteamname", "purpose", "#slack_channel")
mainTeam:addMember(user)

Helper.readK8sResources("k8s_resources/grant_zalando_postgres_access")

Test.gql("Grant postgres access without authorization in non-existent team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		mutation GrantPostgresAccess {
		  grantPostgresAccess(
		    input: {
		      clusterName: "foobar"
		      environmentName: "dev"
		      teamSlug: "non-existing-team"
		      grantee: "some@email.com"
		      duration: "30m"
		    }
		  ) {
		    error
		  }
		}
	]])

	t.check({
		errors = {
			{
				message = Contains('you need the "postgres:access:grant" authorization.'),
				path = {
					"grantPostgresAccess",
				},
			},
		},
		data = Null,
	})
end)

Test.gql("Grant postgres access without authorization in existing team", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query([[
		mutation GrantPostgresAccess {
		  grantPostgresAccess(
		    input: {
		      clusterName: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      grantee: "some@email.com"
		      duration: "30m"
		    }
		  ) {
		    error
		  }
		}
	]])

	t.check({
		errors = {
			{
				message = Contains('you need the "postgres:access:grant" authorization.'),
				path = {
					"grantPostgresAccess",
				},
			},
		},
		data = Null,
	})
end)

Test.gql("Grant postgres access with invalid duration", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		mutation GrantPostgresAccess {
		  grantPostgresAccess(
		    input: {
		      clusterName: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      grantee: "some@email.com"
		      duration: "halfhour"
		    }
		  ) {
		    error
		  }
		}
	]])

	t.check({
		errors = {
			{
				extensions = {
					field = "duration",
				},
				message = Contains('invalid duration "halfhour"'),
				path = {
					"grantPostgresAccess",
				},
			},
		},
		data = Null,
	})
end)

Test.gql("Grant postgres access with out-of-bounds duration", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		mutation GrantPostgresAccess {
		  grantPostgresAccess(
		    input: {
		      clusterName: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      grantee: "some@email.com"
		      duration: "24h"
		    }
		  ) {
		    error
		  }
		}
	]])

	t.check({
		errors = {
			{
				extensions = {
					field = "duration",
				},
				message = Contains('Duration "24h" is out-of-bounds'),
				path = {
					"grantPostgresAccess",
				},
			},
		},
		data = Null,
	})
end)

Test.gql("Grant postgres access to non-existing cluster", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		mutation GrantPostgresAccess {
		  grantPostgresAccess(
		    input: {
		      clusterName: "baz"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      grantee: "some@email.com"
		      duration: "4h"
		    }
		  ) {
		    error
		  }
		}
	]])

	t.check({
		errors = {
			{
				extensions = {
					field = "clusterName",
				},
				message = Contains("Could not find postgres cluster"),
				path = {
					"grantPostgresAccess",
				},
			},
		},
		data = Null,
	})
end)

Test.gql("Grant postgres access with authorization", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		mutation GrantPostgresAccess {
		  grantPostgresAccess(
		    input: {
		      clusterName: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      grantee: "some@email.com"
		      duration: "30m"
		    }
		  ) {
		    error
		  }
		}
	]])

	t.check({
		data = {
			grantPostgresAccess = {
				error = "",
			},
		},
	})
end)

Test.k8s("Validate Role resource", function(t)
	local resourceName = "pg-grant-93a898ea"
	local pgNamespace = string.format("pg-%s", mainTeam:slug())

	t.check("rbac.authorization.k8s.io/v1", "roles", "dev", pgNamespace, resourceName, {
		apiVersion = "rbac.authorization.k8s.io/v1",
		kind = "Role",
		metadata = {
			name = resourceName,
			namespace = pgNamespace,
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["euthanaisa.nais.io/kill-after"] = NotNull(),
				["nais.io/managed-by"] = "console",
				["postgres.data.nais.io/name"] = "foobar",
			},
		},
		rules = {
			{
				apiGroups = {
					"",
				},
				resourceNames = {
					"foobar-0",
					"foobar-1",
					"foobar-2",
				},
				resources = {
					"pods",
				},
				verbs = {
					"get",
					"list",
					"watch",
				},
			},
			{
				apiGroups = {
					"",
				},
				resourceNames = {
					"foobar-0",
					"foobar-1",
					"foobar-2",
				},
				resources = {
					"pods/portforward",
				},
				verbs = {
					"get",
					"list",
					"watch",
					"create",
				},
			},
		},
	})
end)

Test.k8s("Validate RoleBinding resource", function(t)
	local resourceName = "pg-grant-93a898ea"
	local pgNamespace = string.format("pg-%s", mainTeam:slug())

	t.check("rbac.authorization.k8s.io/v1", "rolebindings", "dev", pgNamespace, resourceName, {
		apiVersion = "rbac.authorization.k8s.io/v1",
		kind = "RoleBinding",
		metadata = {
			name = resourceName,
			namespace = pgNamespace,
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["euthanaisa.nais.io/kill-after"] = NotNull(),
				["nais.io/managed-by"] = "console",
				["postgres.data.nais.io/name"] = "foobar",
			},
		},
		roleRef = {
			apiGroup = "rbac.authorization.k8s.io",
			kind = "Role",
			name = resourceName,
		},
		subjects = {
			{
				kind = "User",
				name = "some@email.com",
			},
		},
	})
end)

Test.gql("Check acitivity log entry", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		{
		  team(slug:"someteamname") {
			activityLog {
			  nodes {
				message
				... on PostgresGrantAccessActivityLogEntry {
				  data {
					grantee
					until
				  }
				}
			  }
			}
		  }
		}
	]])

	t.check({
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							message = Contains("Granted access to some@email.com"),
							data = {
								grantee = "some@email.com",
								["until"] = NotNull(),
							},
						},
					},
				},
			},
		},
	})
end)
