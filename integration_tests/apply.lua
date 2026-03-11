local user = User.new("applyer", "apply@example.com", "apply-ext")
local nonMember = User.new("outsider", "outsider@example.com", "outsider-ext")

local team = Team.new("apply-team", "Apply testing", "#apply-team")
team:addMember(user)

Test.rest("create application via apply", function(t)
	t.addHeader("x-user-email", user:email())

	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": [
				{
					"apiVersion": "nais.io/v1alpha1",
					"kind": "Application",
					"metadata": {
						"name": "my-app",
						"namespace": "apply-team"
					},
					"spec": {
						"image": "example.com/my-app:v1",
						"replicas": {
							"min": 1,
							"max": 2
						}
					}
				}
			]
		}
	]])

	t.check(200, {
		results = {
			{
				resource = "Application/my-app",
				namespace = "apply-team",
				environment = "dev",
				status = "created",
			},
		},
	})
end)

Test.k8s("verify resource was created in fake environment", function(t)
	t.check("nais.io/v1alpha1", "applications", "dev", "apply-team", "my-app", {
		apiVersion = "nais.io/v1alpha1",
		kind = "Application",
		metadata = {
			name = "my-app",
			namespace = "apply-team",
		},
		spec = {
			image = "example.com/my-app:v1",
			replicas = {
				min = 1,
				max = 2,
			},
		},
	})
end)

Test.rest("update application via apply", function(t)
	t.addHeader("x-user-email", user:email())

	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": [
				{
					"apiVersion": "nais.io/v1alpha1",
					"kind": "Application",
					"metadata": {
						"name": "my-app",
						"namespace": "apply-team"
					},
					"spec": {
						"image": "example.com/my-app:v2",
						"replicas": {
							"min": 2,
							"max": 4
						}
					}
				}
			]
		}
	]])

	t.check(200, {
		results = {
			{
				resource = "Application/my-app",
				namespace = "apply-team",
				environment = "dev",
				status = "applied",
				changedFields = {
					{
						field = "spec.image",
						oldValue = "example.com/my-app:v1",
						newValue = "example.com/my-app:v2",
					},
					{
						field = "spec.replicas.max",
						oldValue = 2,
						newValue = 4,
					},
					{
						field = "spec.replicas.min",
						oldValue = 1,
						newValue = 2,
					},
				},
			},
		},
	})
end)

Test.k8s("verify resource was updated in fake environment", function(t)
	t.check("nais.io/v1alpha1", "applications", "dev", "apply-team", "my-app", {
		apiVersion = "nais.io/v1alpha1",
		kind = "Application",
		metadata = {
			name = "my-app",
			namespace = "apply-team",
		},
		spec = {
			image = "example.com/my-app:v2",
			replicas = {
				min = 2,
				max = 4,
			},
		},
	})
end)

Test.rest("disallowed resource kind returns 400", function(t)
	t.addHeader("x-user-email", user:email())

	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": [
				{
					"apiVersion": "apps/v1",
					"kind": "Deployment",
					"metadata": {
						"name": "my-deploy",
						"namespace": "apply-team"
					},
					"spec": {}
				}
			]
		}
	]])

	t.check(400, {
		error = Contains("disallowed resource types"),
	})
end)

Test.rest("non-member gets authorization error", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": [
				{
					"apiVersion": "nais.io/v1alpha1",
					"kind": "Application",
					"metadata": {
						"name": "sneaky-app",
						"namespace": "apply-team"
					},
					"spec": {
						"image": "example.com/sneaky:v1"
					}
				}
			]
		}
	]])

	t.check(207, {
		results = {
			{
				resource = "Application/sneaky-app",
				namespace = "apply-team",
				environment = "dev",
				status = "error",
				error = Contains("authorization failed"),
			},
		},
	})
end)

Test.rest("missing environment parameter returns per-resource error", function(t)
	t.addHeader("x-user-email", user:email())

	t.send("POST", "/api/v1/apply", [[
		{
			"resources": [
				{
					"apiVersion": "nais.io/v1alpha1",
					"kind": "Application",
					"metadata": {
						"name": "no-environment-app",
						"namespace": "apply-team"
					},
					"spec": {
						"image": "example.com/app:v1"
					}
				}
			]
		}
	]])

	t.check(207, {
		results = {
			{
				resource = "Application/no-environment-app",
				namespace = "apply-team",
				environment = "",
				status = "error",
				error = Contains("no environment specified"),
			},
		},
	})
end)

Test.rest("empty resources array returns 400", function(t)
	t.addHeader("x-user-email", user:email())

	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": []
		}
	]])

	t.check(400, {
		error = Contains("no resources provided"),
	})
end)

Test.rest("environment annotation overrides query parameter", function(t)
	t.addHeader("x-user-email", user:email())

	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": [
				{
					"apiVersion": "nais.io/v1alpha1",
					"kind": "Application",
					"metadata": {
						"name": "staging-app",
						"namespace": "apply-team",
						"annotations": {
							"nais.io/environment": "staging"
						}
					},
					"spec": {
						"image": "example.com/staging-app:v1"
					}
				}
			]
		}
	]])

	t.check(200, {
		results = {
			{
				resource = "Application/staging-app",
				namespace = "apply-team",
				environment = "staging",
				status = "created",
			},
		},
	})
end)

Test.k8s("verify resource was created in staging environment via annotation", function(t)
	t.check("nais.io/v1alpha1", "applications", "staging", "apply-team", "staging-app", {
		apiVersion = "nais.io/v1alpha1",
		kind = "Application",
		metadata = {
			name = "staging-app",
			namespace = "apply-team",
			annotations = {
				["nais.io/environment"] = "staging",
			},
		},
		spec = {
			image = "example.com/staging-app:v1",
		},
	})
end)

Test.rest("create naisjob via apply", function(t)
	t.addHeader("x-user-email", user:email())

	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": [
				{
					"apiVersion": "nais.io/v1",
					"kind": "Naisjob",
					"metadata": {
						"name": "my-job",
						"namespace": "apply-team"
					},
					"spec": {
						"image": "example.com/my-job:v1",
						"schedule": "0 * * * *"
					}
				}
			]
		}
	]])

	t.check(200, {
		results = {
			{
				resource = "Naisjob/my-job",
				namespace = "apply-team",
				environment = "dev",
				status = "created",
			},
		},
	})
end)

Test.rest("unauthenticated request returns 401", function(t)
	t.send("POST", "/api/v1/apply?environment=dev", [[
		{
			"resources": [
				{
					"apiVersion": "nais.io/v1alpha1",
					"kind": "Application",
					"metadata": {
						"name": "unauth-app",
						"namespace": "apply-team"
					},
					"spec": {}
				}
			]
		}
	]])

	t.check(401, {
		errors = {
			{
				message = "Unauthorized",
			},
		},
	})
end)

Test.gql("activity log contains ApplicationCreatedActivityLogEntry after apply", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format(
		[[
		query {
			team(slug: "%s") {
				activityLog(
					first: 20
					filter: { activityTypes: [RESOURCE_CREATED] }
				) {
					nodes {
						__typename
						message
						actor
						resourceType
						resourceName
						environmentName
						... on ApplicationCreatedActivityLogEntry {
							data {
								apiVersion
								kind
							}
						}
					}
				}
			}
		}
	]],
		team:slug()
	))

	-- RESOURCE_CREATED is registered for both APP and JOB, so the job entry appears too.
	t.check({
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "JobCreatedActivityLogEntry",
							message = "Job my-job created",
							actor = user:email(),
							resourceType = "JOB",
							resourceName = "my-job",
							environmentName = "dev",
						},
						{
							__typename = "ApplicationCreatedActivityLogEntry",
							message = "Application staging-app created",
							actor = user:email(),
							resourceType = "APP",
							resourceName = "staging-app",
							environmentName = "staging",
							data = {
								apiVersion = "nais.io/v1alpha1",
								kind = "Application",
							},
						},
						{
							__typename = "ApplicationCreatedActivityLogEntry",
							message = "Application my-app created",
							actor = user:email(),
							resourceType = "APP",
							resourceName = "my-app",
							environmentName = "dev",
							data = {
								apiVersion = "nais.io/v1alpha1",
								kind = "Application",
							},
						},
					},
				},
			},
		},
	})
end)

Test.gql("activity log contains ApplicationUpdatedActivityLogEntry with changedFields after apply", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format(
		[[
		query {
			team(slug: "%s") {
				activityLog(
					first: 20
					filter: { activityTypes: [RESOURCE_UPDATED] }
				) {
					nodes {
						__typename
						message
						actor
						resourceType
						resourceName
						environmentName
						... on ApplicationUpdatedActivityLogEntry {
							data {
								apiVersion
								kind
								changedFields {
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
	]],
		team:slug()
	))

	t.check({
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "ApplicationUpdatedActivityLogEntry",
							message = "Application my-app updated",
							actor = user:email(),
							resourceType = "APP",
							resourceName = "my-app",
							environmentName = "dev",
							data = {
								apiVersion = "nais.io/v1alpha1",
								kind = "Application",
								changedFields = {
									{
										field = "spec.image",
										oldValue = "example.com/my-app:v1",
										newValue = "example.com/my-app:v2",
									},
									{
										field = "spec.replicas.max",
										oldValue = "2",
										newValue = "4",
									},
									{
										field = "spec.replicas.min",
										oldValue = "1",
										newValue = "2",
									},
								},
							},
						},
					},
				},
			},
		},
	})
end)

Test.gql("activity log contains JobCreatedActivityLogEntry after apply", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format(
		[[
		query {
			team(slug: "%s") {
				activityLog(
					first: 20
					filter: { activityTypes: [RESOURCE_CREATED] }
				) {
					nodes {
						__typename
						resourceType
						resourceName
						environmentName
						... on JobCreatedActivityLogEntry {
							data {
								apiVersion
								kind
							}
						}
					}
				}
			}
		}
	]],
		team:slug()
	))

	-- RESOURCE_CREATED is registered for both JOB and APP, so application entries appear too.
	-- We only assert on the first node which is the most-recently created job.
	t.check({
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "JobCreatedActivityLogEntry",
							resourceType = "JOB",
							resourceName = "my-job",
							environmentName = "dev",
							data = {
								apiVersion = "nais.io/v1",
								kind = "Naisjob",
							},
						},
						{
							__typename = "ApplicationCreatedActivityLogEntry",
							resourceType = "APP",
							resourceName = "staging-app",
							environmentName = "staging",
						},
						{
							__typename = "ApplicationCreatedActivityLogEntry",
							resourceType = "APP",
							resourceName = "my-app",
							environmentName = "dev",
						},
					},
				},
			},
		},
	})
end)
