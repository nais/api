local user = User.new("authenticated", "authenticated@example.com", "someid")
user:admin(true)

local teamSlug = "slug-team"

Test.gql("Create team service account as authenticated user", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		mutation {
			createServiceAccount(
				input: {
					name: "team-sa"
					description: "some description"
				}
			) {
				serviceAccount {
					id
					description
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]])

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("saID"),
					description = "some description",
					roles = {
						nodes = {},
					},
				},
			},
		},
	}
end)

Test.gql("Create service account token", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "my-token"
					description: "some description"
				}
			) {
				secret
				serviceAccount {
					id
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			createServiceAccountToken = {
				secret = Save("token"),
				serviceAccount = {
					id = State.saID,
				},
			},
		},
	}
end)

local sa1HeaderValue = string.format("Bearer %s", State.token)

Test.gql("Create new team as service account without permission", function(t)
	t.addHeader("authorization", sa1HeaderValue)

	t.query(string.format([[
		mutation {
			createTeam(
				input: {
					slug: "%s"
					purpose: "some purpose"
					slackChannel: "#some-channel"
				}
			) {
				team {
					id
					slug
					purpose
					slackChannel
				}
			}
		}
	]], teamSlug))

	t.check {
		data = Null,
		errors = {
			{
				message = "You are authenticated, but your account is not authorized to perform this action. Specifically, you need the \"teams:create\" authorization.",
				path = { "createTeam" },
			},
		},
	}
end)

Test.gql("Assign team creator role to service account as admin", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Team creator",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Create new team as service account", function(t)
	t.addHeader("authorization", sa1HeaderValue)

	t.query(string.format([[
		mutation {
			createTeam(
				input: {
					slug: "%s"
					purpose: "some purpose"
					slackChannel: "#some-channel"
				}
			) {
				team {
					id
					slug
					purpose
					slackChannel
				}
			}
		}
	]], teamSlug))

	t.check {
		data = {
			createTeam = {
				team = {
					id = NotNull(),
					slug = teamSlug,
					purpose = "some purpose",
					slackChannel = "#some-channel",
				},
			},
		},
	}
end)

Test.gql("Add team member without permission", function(t)
	t.addHeader("authorization", sa1HeaderValue)

	Helper.emptyPubSubTopic("topic")

	t.query(string.format([[
		mutation {
			addTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "authenticated@example.com"
					role: MEMBER
				}
			) {
				member {
					role
				}
			}
		}
	]], teamSlug))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"teams:members:admin\" authorization."),
				path = {
					"addTeamMember",
				},
			},
		},
	}
end)

Test.gql("Assign team owner role to service account as admin", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team owner"
				}
			) {
				serviceAccount {
					id
					roles {
						nodes {
							name
						}
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.saID,
					roles = {
						nodes = {
							{
								name = "Team creator",
							},
							{
								name = "Team owner",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Add team member with correct permission", function(t)
	t.addHeader("authorization", sa1HeaderValue)

	Helper.emptyPubSubTopic("topic")

	t.query(string.format([[
		mutation {
			addTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "%s"
					role: MEMBER
				}
			) {
				member {
					role
				}
			}
		}
	]], teamSlug, user:email()))

	t.check {
		data = {
			addTeamMember = {
				member = {
					role = "MEMBER",
				},
			},
		},
	}
end)


Test.gql("Remove team member with correct permission", function(t)
	t.addHeader("authorization", sa1HeaderValue)

	t.query(string.format([[
		mutation {
			removeTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "%s"
				}
			) {
				team {
					members {
						nodes {
							role
						}
					}
				}
			}
		}
	]], teamSlug, user:email()))

	t.check {
		data = {
			removeTeamMember = {
				team = {
					members = {
						nodes = {},
					},
				},
			},
		},
	}
end)
