local user = User.new("user", "user@usersen.com")
local nonMemberUser = User.new("nonmember", "other@user.com")

local mainTeam = Team.new("someteamname", "purpose", "#slack_channel")
mainTeam:addMember(user)
local otherTeam = Team.new("someothername", "purpose", "#slack_channel")

Helper.readK8sResources("k8s_resources/valkey_crud")

Test.gql("Create valkey in non-existing team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateValkey {
		  createValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		      tier: SINGLE_NODE
		      memory: GB_14
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("you need the \"valkeys:create\" authorization."),
				path = {
					"createValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create valkey as non-team member", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation CreateValkey {
		  createValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_14
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"createValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create valkey as team member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateValkey {
		  createValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_14
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

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

Test.gql("Create valkey as team member with existing name", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateValkey {
		  createValkey(
		    input: {
		      name: "not-managed"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_14
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = "Valkey with the name \"not-managed\" already exists, but are not yet managed through Console.",
				path = {
					"createValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.k8s("Validate Valkey resource", function(t)
	t.check("nais.io/v1", "valkeys", "dev", mainTeam:slug(), "foobar", {
		apiVersion = "nais.io/v1",
		kind = "Valkey",
		metadata = {
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = "user@usersen.com",
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
			name = "foobar",
			namespace = "someteamname",
		},
		spec = {
			memory = "14GB",
			tier = "SingleNode",
		},
	})
end)

Test.gql("Update Valkey in non-existing team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateValkey {
		  updateValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		      tier: SINGLE_NODE
		      memory: GB_14
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("you need the \"valkeys:update\" authorization."),
				path = {
					"updateValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update Valkey as non-team-member", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation UpdateValkey {
		  updateValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		      tier: SINGLE_NODE
		      memory: GB_14
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("you need the \"valkeys:update\" authorization."),
				path = {
					"updateValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update Valkey as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateValkey {
		  updateValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_4
		      maxMemoryPolicy: ALLKEYS_RANDOM
		      notifyKeyspaceEvents: "Exd"
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		data = {
			updateValkey = {
				valkey = {
					name = "foobar",
				},
			},
		},
	}
end)

Test.k8s("Validate Valkey resource after update", function(t)
	t.check("nais.io/v1", "valkeys", "dev", mainTeam:slug(), "foobar", {
		apiVersion = "nais.io/v1",
		kind = "Valkey",
		metadata = {
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = "user@usersen.com",
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
			name = "foobar",
			namespace = "someteamname",
		},
		spec = {
			maxMemoryPolicy = "allkeys-random",
			memory = "4GB",
			notifyKeyspaceEvents = "Exd",
			tier = "HighAvailability",
		},
	})
end)

Test.gql("Create valkey with tier and memory equivalent to hobbyist plan", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateValkey {
		  createValkey(
		    input: {
		      name: "foobar-hobbyist"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_1
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		data = {
			createValkey = {
				valkey = {
					name = "foobar-hobbyist",
				},
			},
		},
	}
end)

-- TODO: Do we need this?
Test.k8s("Validate hobbyist Valkey resource", function(t)
	t.check("nais.io/v1", "valkeys", "dev", mainTeam:slug(), "foobar-hobbyist", {
		apiVersion = "nais.io/v1",
		kind = "Valkey",
		metadata = {
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = "user@usersen.com",
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
			name = "foobar-hobbyist",
			namespace = "someteamname",
		},
		spec = {
			memory = "1GB",
			tier = "SingleNode",
		},
	})
end)

Test.gql("List valkeys for team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    valkeys {
		      nodes {
		        name
		        tier
		        memory
		        maxMemoryPolicy
		        notifyKeyspaceEvents
		      }
		    }
		  }
		}
	]]

	t.check {
		data = {
			team = {
				valkeys = {
					nodes = {
						{
							name = "foobar",
							tier = "HIGH_AVAILABILITY",
							memory = "GB_4",
							maxMemoryPolicy = "ALLKEYS_RANDOM",
							notifyKeyspaceEvents = "Exd",
						},
						{
							name = "foobar-hobbyist",
							tier = "SINGLE_NODE",
							memory = "GB_1",
							maxMemoryPolicy = "",
							notifyKeyspaceEvents = "",
						},
						{
							name = "valkey-someteamname-hobbyist-not-managed",
							tier = "SINGLE_NODE",
							memory = "GB_1",
							maxMemoryPolicy = "",
							notifyKeyspaceEvents = "",
						},
						{
							name = "valkey-someteamname-not-managed",
							tier = "SINGLE_NODE",
							memory = "GB_4",
							maxMemoryPolicy = "",
							notifyKeyspaceEvents = "",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Update Valkey with tier and memory equivalent to hobbyist plan", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateValkey {
		  updateValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_1
		      maxMemoryPolicy: ALLKEYS_RANDOM
		      notifyKeyspaceEvents: "Exd"
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]]

	t.check {
		data = {
			updateValkey = {
				valkey = {
					name = "foobar",
				},
			},
		},
	}
end)

Test.k8s("Validate hobbyist Valkey resource after update", function(t)
	t.check("nais.io/v1", "valkeys", "dev", mainTeam:slug(), "foobar", {
		apiVersion = "nais.io/v1",
		kind = "Valkey",
		metadata = {
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = "user@usersen.com",
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
			name = "foobar",
			namespace = "someteamname",
		},
		spec = {
			maxMemoryPolicy = "allkeys-random",
			memory = "1GB",
			notifyKeyspaceEvents = "Exd",
			tier = "SingleNode",
		},
	})
end)

Test.gql("Delete Valkey in non-existing team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation DeleteValkey {
		  deleteValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		    }
		  ) {
		    valkeyDeleted
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("you need the \"valkeys:delete\" authorization."),
				path = {
					"deleteValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Delete Valkey as non-team-member", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation DeleteValkey {
		  deleteValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		    }
		  ) {
		    valkeyDeleted
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"deleteValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Delete Valkey as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation DeleteValkey {
		  deleteValkey(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		    }
		  ) {
		    valkeyDeleted
		  }
		}
	]]

	t.check {
		data = {
			deleteValkey = {
				valkeyDeleted = true,
			},
		},
	}
end)

Test.gql("Verify activity log after deleting valkey", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
		  team(slug: "%s") {
		    activityLog(first: 10, filter: { activityTypes: [VALKEY_DELETED] }) {
		      nodes {
		        __typename
		        message
		        actor
		        createdAt
		        resourceType
		        resourceName
		        ... on ValkeyDeletedActivityLogEntry {
		          environmentName
		          teamSlug
		        }
		      }
		    }
		  }
		}
	]], mainTeam:slug()))

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
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
						},
					},
				},
			},
		},
	}
end)

Test.gql("Verify activity log for valkey operations", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
		  team(slug: "%s") {
		    activityLog(first: 50, filter: { activityTypes: [VALKEY_CREATED, VALKEY_UPDATED, VALKEY_DELETED] }) {
		      nodes {
		        __typename
		        message
		        actor
		        createdAt
		        resourceType
		        resourceName
		        environmentName
		        teamSlug
		        ... on ValkeyUpdatedActivityLogEntry {
		          data {
		            updatedFields {
		              field
		              oldValue
		              newValue
		            }
		          }
		        }
		      }
		    }
		  }
		}
	]], mainTeam:slug()))

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
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
						},
						{
							__typename = "ValkeyUpdatedActivityLogEntry",
							message = "Updated Valkey",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "VALKEY",
							resourceName = "foobar",
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
							data = {
								updatedFields = {
									{
										field = "tier",
										oldValue = "HIGH_AVAILABILITY",
										newValue = "SINGLE_NODE",
									},
									{
										field = "memory",
										oldValue = "GB_4",
										newValue = "GB_1",
									},
								},
							},
						},
						{
							__typename = "ValkeyCreatedActivityLogEntry",
							message = "Created Valkey",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "VALKEY",
							resourceName = "foobar-hobbyist",
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
						},
						{
							__typename = "ValkeyUpdatedActivityLogEntry",
							message = "Updated Valkey",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "VALKEY",
							resourceName = "foobar",
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
							data = {
								updatedFields = {
									{
										field = "tier",
										oldValue = "SINGLE_NODE",
										newValue = "HIGH_AVAILABILITY",
									},
									{
										field = "memory",
										oldValue = "GB_14",
										newValue = "GB_4",
									},
									{
										field = "maxMemoryPolicy",
										oldValue = Null,
										newValue = "ALLKEYS_RANDOM",
									},
									{
										field = "notifyKeyspaceEvents",
										oldValue = Null,
										newValue = "Exd",
									},
								},
							},
						},
						{
							__typename = "ValkeyCreatedActivityLogEntry",
							message = "Created Valkey",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "VALKEY",
							resourceName = "foobar",
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
						},
					},
				},
			},
		},
	}
end)

-- Test cross-team/environment isolation for activity logs
otherTeam:addMember(user)

Test.gql("Create valkey in other team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation CreateValkey {
		  createValkey(
		    input: {
		      name: "other-valkey"
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
	]], otherTeam:slug()))

	t.check {
		data = {
			createValkey = {
				valkey = {
					name = "other-valkey",
				},
			},
		},
	}
end)

Test.gql("Verify otherTeam activity log is isolated from mainTeam", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
		  team(slug: "%s") {
		    activityLog(first: 50, filter: { activityTypes: [VALKEY_CREATED] }) {
		      nodes {
		        __typename
		        message
		        actor
		        resourceType
		        resourceName
		        ... on ValkeyCreatedActivityLogEntry {
		          environmentName
		          teamSlug
		        }
		      }
		    }
		  }
		}
	]], otherTeam:slug()))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "ValkeyCreatedActivityLogEntry",
							message = "Created Valkey",
							actor = user:email(),
							resourceType = "VALKEY",
							resourceName = "other-valkey",
							environmentName = "dev",
							teamSlug = otherTeam:slug(),
						},
					},
				},
			},
		},
	}
end)
