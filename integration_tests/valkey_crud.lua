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
				message = "Resource already exists.",
				path = {
					"createValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.k8s("Validate Valkey resource", function(t)
	local resourceName = string.format("valkey-%s-foobar", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "valkeys", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "Valkey",
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
			plan = "startup-14",
			cloudName = "google-europe-north1",
			terminationProtection = true,
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
		},
	})
end)

Test.k8s("Validate serviceintegration", function(t)
	local resourceName = string.format("valkey-%s-foobar", mainTeam:slug())

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
					kind = "Valkey",
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
	local resourceName = string.format("valkey-%s-foobar", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "valkeys", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "Valkey",
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
			terminationProtection = true,
			userConfig = {
				valkey_maxmemory_policy = "allkeys-random",
				valkey_notify_keyspace_events = "Exd",
			},
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
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

Test.k8s("Validate hobbyist Valkey resource", function(t)
	local resourceName = string.format("valkey-%s-foobar-hobbyist", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "valkeys", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "Valkey",
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
			terminationProtection = true,
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
		},
	})
end)

Test.k8s("Validate hobbyist serviceintegration", function(t)
	local resourceName = string.format("valkey-%s-foobar-hobbyist", mainTeam:slug())

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
					kind = "Valkey",
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

Test.gql("Update non-console managed Valkey as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateValkey {
		  updateValkey(
		    input: {
		      name: "not-managed"
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
		errors = {
			{
				message = "Valkey someteamname/not-managed is not managed by Console",
				path = {
					"updateValkey",
				},
			},
		},
		data = Null,
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
	local resourceName = string.format("valkey-%s-foobar", mainTeam:slug())

	t.check("aiven.io/v1alpha1", "valkeys", "dev", mainTeam:slug(), resourceName, {
		apiVersion = "aiven.io/v1alpha1",
		kind = "Valkey",
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
			terminationProtection = true,
			userConfig = {
				valkey_maxmemory_policy = "allkeys-random",
				valkey_notify_keyspace_events = "Exd",
			},
			tags = {
				environment = "dev",
				team = mainTeam:slug(),
				tenant = "some-tenant",
			},
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

Test.gql("Delete non-managed valkey as team-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation DeleteValkey {
		  deleteValkey(
		    input: {
		      name: "not-managed"
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
				message = "Valkey someteamname/not-managed is not managed by Console",
				path = {
					"deleteValkey",
				},
			},
		},
		data = Null,
	}
end)
