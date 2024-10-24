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
					slug = "myteam",
				},
			},
		},
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
					"createSecret",
				},
			},
		},
		data = Null,
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
					values = {},
				},
			},
		},
	}
end)

Test.gql("Add secret value", function(t)
	t.query [[
		mutation {
			addSecretValue(input: {
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
			addSecretValue = {
				secret = {
					name = "secret-name",
					values = {
						{
							name = "value-name",
							value = "value",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Add secret value that already exists", function(t)
	t.query [[
		mutation {
			addSecretValue(input: {
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
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = "The secret already contains a secret value with the name \"value-name\".",
				path = {
					"addSecretValue",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update secret value", function(t)
	t.query [[
		mutation {
			updateSecretValue(input: {
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
			updateSecretValue = {
				secret = {
					name = "secret-name",
					values = {
						{
							name = "value-name",
							value = "new value",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Update secret value that does not exist", function(t)
	t.query [[
		mutation {
			updateSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "does-not-exist",
					value: "new value"
				}
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = "The secret does not contain a secret value with the name \"does-not-exist\".",
				path = {
					"updateSecretValue",
				},
			},
		},
		data = Null,
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
				message = "The secret does not contain a secret value with the name: \"foobar\".",
				path = {
					"removeSecretValue",
				},
			},
		},
	}
end)

Test.gql("Remove secret value that already exists", function(t)
	t.query [[
		mutation {
			addSecretValue(input: {
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
			addSecretValue = {
				secret = {
					name = "secret-name",
					values = {
						{
							name = "dont-remove",
						},
						{
							name = "value-name",
						},
					},
				},
			},
		},
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
							name = "dont-remove",
						},
					},
				},
			},
		},
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
					"deleteSecret",
				},
			},
		},
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
				secretDeleted = true,
			},
		},
	}
end)
