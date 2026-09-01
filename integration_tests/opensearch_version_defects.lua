local user = User.new("user", "user@usersen.com")
local team = Team.new("myteam", "purpose", "#slack_channel")
team:addMember(user)

Helper.readK8sResources("k8s_resources/opensearch_version")
Helper.setAivenVersion("opensearch-myteam-crpin", "2.19.3")

-- Aiven is the sole authority for the version. The CR pin never feeds what the API
-- reports, and a version the API cannot source from Aiven is not reported at all.
local unreportedVersion =
"The server errored out while processing your request, and we didn't write a suitable error message. You might consider that a bug on our side. Please try again, and if the error persists, contact the Nais team."

local function versionQuery(name)
	return string.format([[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					openSearch(name: "%s") {
						name
						version {
							actual
							desiredMajor
						}
					}
				}
			}
		}
	]], name)
end

Test.gql("Aiven's version wins over a CR that pins a different one", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(versionQuery("crpin"))

	t.check {
		data = {
			team = {
				environment = {
					openSearch = {
						name = "crpin",
						version = {
							actual = "2.19.3",
							desiredMajor = "V2_19",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Aiven reports no version, CR pins one", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(versionQuery("nometa"))

	t.check {
		data = {
			team = {
				environment = {
					openSearch = {
						name = "nometa",
						version = {
							actual = Null,
							desiredMajor = "V2_19",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Neither Aiven nor the CR reports a version", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(versionQuery("nocrd"))

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = unreportedVersion,
				path = {
					"team",
					"environment",
					"openSearch",
					"version",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update OpenSearch when no version is known anywhere", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "nocrd"
		      environmentName: "dev"
		      teamSlug: "myteam"
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
		errors = {
			{
				locations = NotNull(),
				message = unreportedVersion,
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Aiven silent, the CR's pinned version does not rescue the upgrade", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "nometa"
		      environmentName: "dev"
		      teamSlug: "myteam"
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
		errors = {
			{
				locations = NotNull(),
				message = unreportedVersion,
				path = {
					"updateOpenSearch",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Upgrade OpenSearch to a version on the upgrade path", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation UpdateOpenSearch {
		  updateOpenSearch(
		    input: {
		      name: "crpin"
		      environmentName: "dev"
		      teamSlug: "myteam"
		      tier: SINGLE_NODE
		      memory: GB_2
		      version: V3_6
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
					name = "crpin",
				},
			},
		},
	}
end)

Test.gql("Creating without a version uses the newest Aiven supports", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreateOpenSearch {
		  createOpenSearch(
		    input: {
		      name: "newest"
		      environmentName: "dev"
		      teamSlug: "myteam"
		      tier: SINGLE_NODE
		      memory: GB_2
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
					name = "newest",
				},
			},
		},
	}
end)

Test.k8s("The created instance pins the newest version", function(t)
	t.check("aiven.io/v1alpha1", "opensearches", "dev", "myteam", "opensearch-myteam-newest", {
		apiVersion = "aiven.io/v1alpha1",
		kind = "OpenSearch",
		metadata = {
			name = "opensearch-myteam-newest",
			namespace = "myteam",
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
				team = "myteam",
				tenant = "some-tenant",
			},
			userConfig = {
				opensearch_version = "3.6",
			},
		},
	})
end)
