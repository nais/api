local user = User.new("user", "user@usersen.com")
local team = Team.new("myteam", "purpose", "#slack_channel")
team:addMember(user)

Helper.readK8sResources("k8s_resources/valkey_version")
-- "versioned" sits on the newest rung, "upgradable" on the oldest, so both ends of the
-- upgrade path have a subject. Both pin 8.1 in their CR, which Aiven overrides.
Helper.setAivenVersion("valkey-myteam-versioned", "9.1.0")
Helper.setAivenVersion("valkey-myteam-upgradable", "8.1.4")

-- Aiven is the authority for the version, mirroring OpenSearch. A version the API
-- cannot source from Aiven is not reported at all.
local unreportedVersion =
"The server errored out while processing your request, and we didn't write a suitable error message. You might consider that a bug on our side. Please try again, and if the error persists, contact the Nais team."

local function versionQuery(name)
	return string.format([[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					valkey(name: "%s") {
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
	t.query(versionQuery("versioned"))

	t.check {
		data = {
			team = {
				environment = {
					valkey = {
						name = "versioned",
						version = {
							actual = "9.1.0",
							desiredMajor = "V9_1",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Aiven reports no Valkey version", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(versionQuery("nometa"))

	t.check {
		data = {
			team = {
				environment = {
					valkey = {
						name = "nometa",
						version = {
							actual = Null,
							desiredMajor = "V8_1",
						},
					},
				},
			},
		},
	}
end)

local function updateVersion(name, version)
	return string.format([[
		mutation UpdateValkey {
		  updateValkey(
		    input: {
		      name: "%s"
		      environmentName: "dev"
		      teamSlug: "myteam"
		      tier: SINGLE_NODE
		      memory: GB_1
		      version: %s
		    }
		  ) {
		    valkey {
		      name
		    }
		  }
		}
	]], name, version)
end

Test.gql("Requesting the version Aiven already reports changes nothing", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(updateVersion("versioned", "V9_1"))

	t.check {
		data = {
			updateValkey = {
				valkey = {
					name = "versioned",
				},
			},
		},
	}
end)

Test.gql("Reject a Valkey downgrade below the version Aiven reports", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(updateVersion("versioned", "V8_1"))

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = "Cannot change Valkey version from V9_1 to V8_1. No further upgrades available.",
				path = {
					"updateValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Aiven silent, the CR's pinned version does not rescue the upgrade", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(updateVersion("nometa", "V9_1"))

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = unreportedVersion,
				path = {
					"updateValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update Valkey when no version is known anywhere", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(updateVersion("nocrd", "V9_1"))

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = unreportedVersion,
				path = {
					"updateValkey",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Upgrade Valkey to a version on the upgrade path", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(updateVersion("upgradable", "V9_1"))

	t.check {
		data = {
			updateValkey = {
				valkey = {
					name = "upgradable",
				},
			},
		},
	}
end)
