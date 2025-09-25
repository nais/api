local user = User.new("user", "user@usersen.com")
local nonMemberUser = User.new("nonmember", "other@user.com")

local mainTeam = Team.new("someteamname", "purpose", "#slack_channel")
mainTeam:addMember(user)

Helper.readK8sResources("k8s_resources/opensearch_crud")

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
		      memory: RAM_16GB
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
		      memory: RAM_16GB
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
		      memory: RAM_16GB
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
		      memory: RAM_16GB
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
		      memory: RAM_2GB
		      version: V2
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
				message = "Invalid OpenSearch memory for tier. HIGH_AVAILABILITY cannot have memory RAM_2GB",
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
		      memory: RAM_4GB
		      version: V2
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
				message = "Storage capacity must be between 240G and 1200G for tier \"HIGH_AVAILABILITY\" and memory \"RAM_4GB\".",
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
				opensearch_version = "2",
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
		      memory: RAM_2GB
		      version: V2
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
				opensearch_version = "2",
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
		      memory: RAM_16GB
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
		      memory: RAM_16GB
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
		      memory: RAM_4GB
		      version: V2
		      storageGB: 1000
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
			disk_space = "1000G",
			terminationProtection = true,
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
			userConfig = {
				opensearch_version = "2",
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
							name = "foobar",
							tier = "HIGH_AVAILABILITY",
							memory = "RAM_4GB",
						},
						{
							name = "foobar-hobbyist",
							tier = "SINGLE_NODE",
							memory = "RAM_2GB",
						},
						{
							name = "noversion",
							tier = "SINGLE_NODE",
							memory = "RAM_2GB",
						},
						{
							name = "opensearch-someteamname-hobbyist-not-managed",
							tier = "SINGLE_NODE",
							memory = "RAM_2GB",
						},
						{
							name = "opensearch-someteamname-not-managed",
							tier = "HIGH_AVAILABILITY",
							memory = "RAM_8GB",
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
		      name: "foobar"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: RAM_4GB
		      version: V1
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
				message = "Cannot downgrade OpenSearch version from V2 to V1",
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Downgrade OpenSearch without explicit version set", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "noversion"
		      environmentName: "dev"
		      teamSlug: "someteamname"
		      tier: HIGH_AVAILABILITY
		      memory: RAM_4GB
		      version: V1
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
				message = "Cannot downgrade OpenSearch version from V2 to V1",
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
		      memory: RAM_4GB
		      version: V2
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
		      memory: RAM_2GB
		      version: V2
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
				opensearch_version = "2",
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
				message = Contains("you need the \"opensearches:delete\" authorization."),
				path = {
					"deleteOpenSearch",
				},
			},
		},
		data = Null,
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
				message = "OpenSearch someteamname/not-managed is not managed by Console",
				path = {
					"deleteOpenSearch",
				},
			},
		},
		data = Null,
	}
end)
