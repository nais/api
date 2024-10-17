Test.gql("Create team", function(t)
	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "myteam"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					id
					slug
				}
			}
		}
	]]

	t.check {
		data = {
			createTeam = {
				team = {
					id = Save("teamID"),
					slug = "myteam"
				}
			}
		}
	}
end)

Test.gql("Create secret for team that does not exist", function(t)
	t.query [[
		mutation {
			createSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "team-that-does-not-exist"
			}) {
				secret {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"createSecret"
				}
			},
		},
		data = Null
	}
end)

Test.gql("Create secret", function(t)
	t.query [[
		mutation {
			createSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
			}) {
				secret {
					name
					values {
						name
					}
				}
			}
		}
	]]

	t.check {
		data = {
			createSecret = {
				secret = {
					name = "secret-name",
					values = {}
				}
			}
		}
	}
end)

Test.gql("Set secret value that does not exist (create)", function(t)
	t.query [[
		mutation {
			setSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "value-name",
					value: "value"
				}
			}) {
				secret {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			setSecretValue = {
				secret = {
					name = "secret-name",
					values = {
						{
							name = "value-name",
							value = "value"
						}
					}
				}
			}
		}
	}
end)

Test.gql("Set secret value that already exists (update)", function(t)
	t.query [[
		mutation {
			setSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "value-name",
					value: "new value"
				}
			}) {
				secret {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			setSecretValue = {
				secret = {
					name = "secret-name",
					values = {
						{
							name = "value-name",
							value = "new value"
						}
					}
				}
			}
		}
	}
end)

Test.gql("Remove secret value that does not exist", function(t)
	t.query [[
		mutation {
			removeSecretValue(input: {
				secretName: "secret-name"
				environment: "dev"
				team: "myteam"
				valueName: "foobar"
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = "No such secret value exists: \"foobar\"",
				path = {
					"removeSecretValue"
				}
			}
		}
	}
end)

Test.gql("Remove secret value that already exists", function(t)
	t.query [[
		mutation {
			setSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "dont-remove",
					value: "secret"
				}
			}) {
				secret {
					name
					values {
						name
					}
				}
			}
		}
	]]

	t.check {
		data = {
			setSecretValue = {
				secret = {
					name = "secret-name",
					values = {
						{
							name = "dont-remove"
						},
						{
							name = "value-name"
						}
					}
				}
			}
		}
	}

	t.query [[
		mutation {
			removeSecretValue(input: {
				secretName: "secret-name"
				environment: "dev"
				team: "myteam"
				valueName: "value-name"
			}) {
				secret {
					name
					values {
						name
					}
				}
			}
		}
	]]

	t.check {
		data = {
			removeSecretValue = {
				secret = {
					name = "secret-name",
					values = {
						{
							name = "dont-remove"
						}
					}
				}
			}
		}
	}
end)

Test.gql("Delete secret that does not exist", function(t)
	t.query [[
		mutation {
			deleteSecret(input: {
				name: "secret-name-that-does-not-exist"
				environment: "dev"
				team: "myteam"
			}) {
				secretDeleted
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Resource not found"),
				path = {
					"deleteSecret"
				}
			}
		}
	}
end)

Test.gql("Delete secret that exists", function(t)
	t.query [[
		mutation {
			deleteSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
			}) {
				secretDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteSecret = {
				secretDeleted = true
			}
		}
	}
end)
