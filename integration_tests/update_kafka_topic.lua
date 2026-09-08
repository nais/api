local user = User.new("kafka-topic-member", "kafka-topic-member@usersen.com")
local nonMember = User.new("kafka-topic-non-member", "kafka-topic-non-member@usersen.com")
local team = Team.new("kafka-update-team", "purpose", "#slack-channel")
team:addMember(user)

Helper.readK8sResources("k8s_resources/update_kafka_topic")

Test.gql("update Kafka topic rejects non-members", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query [[
		mutation {
			updateKafkaTopic(input: {
				name: "orders"
				teamSlug: "kafka-update-team"
				environmentName: "dev"
				addGrants: [{ subject: "orders-api", teamName: "consumer-team", access: READ }]
			}) {
				kafkaTopic { name }
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("you need the \"kafka:update\" authorization"),
				path = { "updateKafkaTopic" },
			},
		},
	}
end)

Test.gql("update Kafka topic adds grants", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateKafkaTopic(input: {
				name: "orders"
				teamSlug: "kafka-update-team"
				environmentName: "dev"
				addGrants: [
					{ subject: "orders-api", teamName: "consumer-team", access: READ }
					{ subject: "orders-writer", teamName: "consumer-team", access: READWRITE }
				]
			}) {
				kafkaTopic {
					name
					acl { nodes { workloadName teamName access } }
				}
			}
		}
	]]

	t.check {
		data = {
			updateKafkaTopic = {
				kafkaTopic = {
					name = "orders",
					acl = {
						nodes = {
							{ workloadName = "orders-api",    teamName = "consumer-team", access = "READ" },
							{ workloadName = "orders-writer", teamName = "consumer-team", access = "READWRITE" },
						},
					},
				},
			},
		},
	}
end)

Test.k8s("Kafka topic contains added grants", function(t)
	t.check("kafka.nais.io/v1", "topics", "dev", team:slug(), "orders", {
		apiVersion = "kafka.nais.io/v1",
		kind = "Topic",
		metadata = Ignore(),
		spec = {
			acl = {
				{ access = "read",      application = "orders-api",    team = "consumer-team" },
				{ access = "readwrite", application = "orders-writer", team = "consumer-team" },
			},
			pool = "dev",
		},
	})
end)

Test.gql("update Kafka topic ignores existing grants", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateKafkaTopic(input: {
				name: "orders"
				teamSlug: "kafka-update-team"
				environmentName: "dev"
				addGrants: [{ subject: "orders-api", teamName: "consumer-team", access: READ }]
			}) {
				kafkaTopic {
					acl {
						nodes {
							workloadName
							teamName
							access
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			updateKafkaTopic = {
				kafkaTopic = {
					acl = {
						nodes = {
							{ workloadName = "orders-api",    teamName = "consumer-team", access = "READ" },
							{ workloadName = "orders-writer", teamName = "consumer-team", access = "READWRITE" },
						},
					},
				},
			},
		},
	}
end)

Test.gql("update Kafka topic add yet another grants", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateKafkaTopic(input: {
				name: "orders"
				teamSlug: "kafka-update-team"
				environmentName: "dev"
				addGrants: [
					{ subject: "orders-admin", teamName: "consumer-team", access: READWRITE }
				]
			}) {
				kafkaTopic {
					name
					acl { nodes { workloadName teamName access } }
				}
			}
		}
	]]

	t.check {
		data = {
			updateKafkaTopic = {
				kafkaTopic = {
					name = "orders",
					acl = {
						nodes = {
							{ workloadName = "orders-admin",  teamName = "consumer-team", access = "READWRITE" },
							{ workloadName = "orders-api",    teamName = "consumer-team", access = "READ" },
							{ workloadName = "orders-writer", teamName = "consumer-team", access = "READWRITE" },
						},
					},
				},
			},
		},
	}
end)

Test.k8s("Kafka topic contains all the added grants", function(t)
	t.check("kafka.nais.io/v1", "topics", "dev", team:slug(), "orders", {
		apiVersion = "kafka.nais.io/v1",
		kind = "Topic",
		metadata = Ignore(),
		spec = {
			acl = {
				{ access = "read",      application = "orders-api",    team = "consumer-team" },
				{ access = "readwrite", application = "orders-writer", team = "consumer-team" },
				{ access = "readwrite", application = "orders-admin",  team = "consumer-team" },
			},
			pool = "dev",
		},
	})
end)
