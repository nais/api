local user = User.new("user", "user@usersen.com")
local nonMemberUser = User.new("nonmember", "other@user.com")

local mainTeam = Team.new("someteamname", "purpose", "#slack_channel")
mainTeam:addMember(user)

Helper.readK8sResources("k8s_resources/opensearch_crud")
-- The instances these tests update run the version Aiven reports back. Without it every
-- update would fail on an unknown current version. The two instances above 2.19 exist so
-- a downgrade can be attempted at all: nothing below 2.19 is selectable any more.
Helper.setAivenVersion("opensearch-someteamname-foobar", "2.19.3")
Helper.setAivenVersion("opensearch-someteamname-noversion", "3.3.0")
Helper.setAivenVersion("opensearch-someteamname-downgrade", "3.6.0")

Test.gql("Create opensearch in non-existing team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2_19
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains("you need the \"opensearches:create\" authorization."),
				path = {
					"createOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create opensearch as non-team member", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2_19
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains("You are authenticated"),
				path = {
					"createOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create opensearch as team member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2_19
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		data = {
			createOpenSearch = {
				openSearch = {
					name = "foobar",
				},
			},
		},
	}
end)

Test.gql("Create opensearch as team member with existing name", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "not-managed"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2_19
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = "Resource already exists.",
				path = {
					"createOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create opensearch with invalid tier and memory combination", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_2
		      version: V2_19
		      storageGB: 16
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				extensions = {
					field = "memory",
				},
				message = "Invalid OpenSearch memory for tier. HIGH_AVAILABILITY cannot have memory GB_2",
				path = {
					"createOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create opensearch with invalid storage capacity", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_4
		      version: V2_19
		      storageGB: 16
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				extensions = {
					field = "storageGB",
				},
				message = "Storage capacity for tier \"HIGH_AVAILABILITY\" and memory \"GB_4\" must be in the range [240, 1200] in increments of 30. Examples: [240, 270, 300, ...]",
				path = {
					"createOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create opensearch with invalid storage capacity increment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_8
		      version: V2_19
		      storageGB: 180
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				extensions = {
					field = "storageGB",
				},
				message = "Storage capacity for tier \"SINGLE_NODE\" and memory \"GB_8\" must be in the range [175, 875] in increments of 10. Examples: [175, 185, 195, ...]",
				path = {
					"createOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.k8s("Validate OpenSearch resource", function(t)
	local resourceName = string.format("opensearch-%s-foobar", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "opensearches", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "OpenSearch",
		metadata = {
			name = resourceName,
			namespace = mainTeam:slug(),
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
		},
		spec = {
			project = "aiven-dev",
			projectVpcId = "aiven-vpc",
			plan = "startup-16",
			cloudName = "google-europe-north1",
			disk_space = "350G",
			terminationProtection = true,
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
			userConfig = {
				opensearch_version = "2.19",
			},
		},
	})
end)

Test.k8s("Validate serviceintegration", function(t)
	local resourceName = string.format("opensearch-%s-foobar", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "serviceintegrations", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "ServiceIntegration",
		metadata = {
			name = resourceName,
			namespace = mainTeam:slug(),
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
			ownerReferences = {
				{
					apiVersion = "aiven.io/v1alpha1",
					kind = "OpenSearch",
					name = resourceName,
					uid = NotNull(),
				},
			},
		},
		spec = {
			project = "aiven-dev",
			destinationEndpointId = "endpoint-id",
			integrationType = "prometheus",
			sourceServiceName = resourceName,
		},
	})
end)

Test.gql("Create opensearch with tier and memory equivalent to hobbyist plan", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "foobar-hobbyist"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_2
		      version: V2_19
		      storageGB: 16
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		data = {
			createOpenSearch = {
				openSearch = {
					name = "foobar-hobbyist",
				},
			},
		},
	}
end)

Test.k8s("Validate hobbyist OpenSearch resource", function(t)
	local resourceName = string.format("opensearch-%s-foobar-hobbyist", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "opensearches", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "OpenSearch",
		metadata = {
			name = resourceName,
			namespace = mainTeam:slug(),
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
		},
		spec = {
			project = "aiven-dev",
			projectVpcId = "aiven-vpc",
			plan = "hobbyist",
			cloudName = "google-europe-north1",
			disk_space = "16G",
			terminationProtection = true,
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
			userConfig = {
				opensearch_version = "2.19",
			},
		},
	})
end)

Test.k8s("Validate hobbyist serviceintegration", function(t)
	local resourceName = string.format("opensearch-%s-foobar-hobbyist", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "serviceintegrations", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "ServiceIntegration",
		metadata = {
			name = resourceName,
			namespace = mainTeam:slug(),
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
			ownerReferences = {
				{
					apiVersion = "aiven.io/v1alpha1",
					kind = "OpenSearch",
					name = resourceName,
					uid = NotNull(),
				},
			},
		},
		spec = {
			project = "aiven-dev",
			destinationEndpointId = "endpoint-id",
			integrationType = "prometheus",
			sourceServiceName = resourceName,
		},
	})
end)

Test.gql("Update OpenSearch in non-existing team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2_19
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains("you need the \"opensearches:update\" authorization."),
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update OpenSearch as non-team-member", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2_19
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains("you need the \"opensearches:update\" authorization."),
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update OpenSearch as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_4
		      version: V2_19
		      storageGB: 1020
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		data = {
			updateOpenSearch = {
				openSearch = {
					name = "foobar",
				},
			},
		},
	}
end)

Test.k8s("Validate OpenSearch resource after update", function(t)
	local resourceName = string.format("opensearch-%s-foobar", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "opensearches", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "OpenSearch",
		metadata = {
			name = resourceName,
			namespace = mainTeam:slug(),
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
		},
		spec = {
			project = "aiven-dev",
			projectVpcId = "aiven-vpc",
			plan = "business-4",
			cloudName = "google-europe-north1",
			disk_space = "1020G",
			terminationProtection = true,
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
			userConfig = {
				opensearch_version = "2.19",
			},
		},
	})
end)

Test.gql("List opensearches for team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    openSearches {
		      nodes {
		        name
		        tier
		        memory
		      }
		    }
		  }
		}
	]]

	t.check {
		data = {
			team = {
				openSearches = {
					nodes = {
						{
							name = "downgrade",
							tier = "SINGLE_NODE",
							memory = "GB_2",
						},
						{
							name = "foobar",
							tier = "HIGH_AVAILABILITY",
							memory = "GB_4",
						},
						{
							name = "foobar-hobbyist",
							tier = "SINGLE_NODE",
							memory = "GB_2",
						},
						{
							name = "noversion",
							tier = "SINGLE_NODE",
							memory = "GB_2",
						},
						{
							name = "opensearch-someteamname-hobbyist-not-managed",
							tier = "SINGLE_NODE",
							memory = "GB_2",
						},
						{
							name = "opensearch-someteamname-not-managed",
							tier = "HIGH_AVAILABILITY",
							memory = "GB_8",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Downgrade OpenSearch as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "downgrade"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_4
		      version: V2_19
		      storageGB: 240
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = "Cannot change OpenSearch version from V3_6 to V2_19. No further upgrades available.",
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Downgrade OpenSearch without a version pinned in the CR", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "noversion"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_4
		      version: V2_19
		      storageGB: 240
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = "Cannot change OpenSearch version from V3_3 to V2_19. New version must be one of [V3_6]",
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Reject a deprecated OpenSearch version on create", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "deprecated-create"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				extensions = {
					field = "version",
				},
				message = "OpenSearch version V2 is deprecated: use V2_19 instead.",
				path = {
					"createOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Reject a deprecated OpenSearch version on update", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_4
		      version: V3_3
		      storageGB: 240
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				extensions = {
					field = "version",
				},
				message = "OpenSearch version V3_3 is deprecated: vendor support disappears 2027-02-01.",
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update non-console managed OpenSearch as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "not-managed"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: GB_4
		      version: V2_19
		      storageGB: 240
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = "OpenSearch someteamname/not-managed is not managed by Console",
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update OpenSearch with tier and memory equivalent to hobbyist plan", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: SINGLE_NODE
		      memory: GB_2
		      version: V2_19
		      storageGB: 16
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]]

	t.check {
		data = {
			updateOpenSearch = {
				openSearch = {
					name = "foobar",
				},
			},
		},
	}
end)

Test.k8s("Validate hobbyist OpenSearch resource after update", function(t)
	local resourceName = string.format("opensearch-%s-foobar", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "opensearches", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "OpenSearch",
		metadata = {
			name = resourceName,
			namespace = mainTeam:slug(),
			annotations = {
				["console.nais.io/last-modified-at"] = NotNull(),
				["console.nais.io/last-modified-by"] = user:email(),
			},
			labels = {
				["app.kubernetes.io/managed-by"] = "console",
				["nais.io/managed-by"] = "console",
			},
		},
		spec = {
			project = "aiven-dev",
			projectVpcId = "aiven-vpc",
			plan = "hobbyist",
			cloudName = "google-europe-north1",
			disk_space = "16G",
			terminationProtection = true,
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
			userConfig = {
				opensearch_version = "2.19",
			},
		},
	})
end)

Test.gql("Delete OpenSearch in non-existing team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation DeleteOpenSearch {
		  deleteOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "devteam"
		    }
		  ) {
				openSearchDeleted
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains("you need the \"opensearches:delete\" authorization."),
				path = {
					"deleteOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

-- Test cross-team/environment isolation for activity logs
local otherTeam = Team.new("otherteamname", "purpose", "#slack_channel")
otherTeam:addMember(user)

Test.gql("Create opensearch in other team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "other-opensearch"
		      environmentName: "dev"
		      teamSlug: "%s"
		      tier: SINGLE_NODE
		      memory: GB_16
		      version: V2_19
		      storageGB: 350
		    }
		  ) {
		    openSearch {
		      name
		    }
		  }
		}
	]], otherTeam:slug()))

	t.check {
		data = {
			createOpenSearch = {
				openSearch = {
					name = "other-opensearch",
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
		    activityLog(first: 50, filter: { activityTypes: [OPENSEARCH_CREATED] }) {
		      nodes {
		        __typename
		        message
		        actor
		        resourceType
		        resourceName
		        ... on OpenSearchCreatedActivityLogEntry {
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
							__typename = "OpenSearchCreatedActivityLogEntry",
							message = "Created OpenSearch",
							actor = user:email(),
							resourceType = "OPENSEARCH",
							resourceName = "other-opensearch",
							environmentName = "dev",
							teamSlug = otherTeam:slug(),
						},
					},
				},
			},
		},
	}
end)

Test.gql("Delete OpenSearch as non-team-member", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation DeleteOpenSearch {
		  deleteOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		    }
		  ) {
				openSearchDeleted
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains("You are authenticated"),
				path = {
					"deleteOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Delete OpenSearch as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation DeleteOpenSearch {
		  deleteOpenSearch(
		    input: {
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		    }
		  ) {
				openSearchDeleted
		  }
		}
	]]

	t.check {
		data = {
			deleteOpenSearch = {
				openSearchDeleted = true,
			},
		},
	}
end)

Test.gql("Verify activity log for opensearch operations", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
		  team(slug: "%s") {
		    activityLog(first: 50, filter: { activityTypes: [OPENSEARCH_CREATED, OPENSEARCH_UPDATED, OPENSEARCH_DELETED] }) {
		      nodes {
		        __typename
		        message
		        actor
		        createdAt
		        resourceType
		        resourceName
		        environmentName
		        teamSlug
		        ... on OpenSearchUpdatedActivityLogEntry {
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
							__typename = "OpenSearchDeletedActivityLogEntry",
							message = "Deleted OpenSearch",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "OPENSEARCH",
							resourceName = "foobar",
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
						},
						{
							__typename = "OpenSearchUpdatedActivityLogEntry",
							message = "Updated OpenSearch",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "OPENSEARCH",
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
										newValue = "GB_2",
									},
									{
										field = "storageGB",
										oldValue = "1020",
										newValue = "16",
									},
								},
							},
						},
						{
							__typename = "OpenSearchUpdatedActivityLogEntry",
							message = "Updated OpenSearch",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "OPENSEARCH",
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
										oldValue = "GB_16",
										newValue = "GB_4",
									},
									{
										field = "storageGB",
										oldValue = "350",
										newValue = "1020",
									},
								},
							},
						},
						{
							__typename = "OpenSearchCreatedActivityLogEntry",
							message = "Created OpenSearch",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "OPENSEARCH",
							resourceName = "foobar-hobbyist",
							environmentName = "dev",
							teamSlug = mainTeam:slug(),
						},
						{
							__typename = "OpenSearchCreatedActivityLogEntry",
							message = "Created OpenSearch",
							actor = user:email(),
							createdAt = NotNull(),
							resourceType = "OPENSEARCH",
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

Test.gql("Delete non-managed opensearch as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation DeleteOpenSearch {
		  deleteOpenSearch(
		    input: {
		      name: "not-managed"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		    }
		  ) {
				openSearchDeleted
		  }
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = "OpenSearch someteamname/not-managed is not managed by Console",
				path = {
					"deleteOpenSearch",
				},
			},
		},
		data = Null,
	}
end)
