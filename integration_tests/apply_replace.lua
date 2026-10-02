local user = User.new()
local team = Team.new("apply-replace-team", "Full-object apply testing", "#apply-replace-team")
team:addMember(user)

Test.rest("create resources with fields that will be omitted", function(t)
	t.addHeader("x-user-email", user:email())
	t.send("POST", "/api/v1/teams/apply-replace-team/environments/dev/apply", [[
		{"resources": [
			{
				"apiVersion": "nais.io/v1alpha1",
				"kind": "Application",
				"metadata": {
					"name": "my-app",
					"labels": {"kept": "same", "removed": "old"},
					"annotations": {"removed": "old"}
				},
				"spec": {
					"image": "example.com/my-app:v1",
					"resources": {
						"limits": {"cpu": "1", "memory": "512Mi"},
						"requests": {"cpu": "100m", "memory": "256Mi"}
					}
				}
			},
			{
				"apiVersion": "v1",
				"kind": "ConfigMap",
				"metadata": {"name": "my-config"},
				"data": {"kept": "old", "removed": "old"},
				"binaryData": {"removed": "b2xk"}
			}
		]}
	]])
	t.check(200, {
		results = {
			{ resource = "Application/my-app",  environmentName = "dev", status = "created" },
			{ resource = "ConfigMap/my-config", environmentName = "dev", status = "created" },
		},
	})
end)

local replacement = [[
	{"resources": [
		{
			"apiVersion": "nais.io/v1alpha1",
			"kind": "Application",
			"metadata": {
				"name": "my-app",
				"labels": {"kept": "same"}
			},
			"spec": {
				"image": "example.com/my-app:v1",
				"resources": {
					"limits": {"memory": "512Mi"},
					"requests": {"cpu": "100m", "memory": "256Mi"}
				}
			}
		},
		{
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"metadata": {"name": "my-config"},
			"data": {"kept": "new"}
		}
	]}
]]

Test.rest("apply replaces the entire object and reports removed fields", function(t)
	t.addHeader("x-user-email", user:email())
	t.send("POST", "/api/v1/teams/apply-replace-team/environments/dev/apply", replacement)
	t.check(200, {
		results = {
			{
				resource = "Application/my-app",
				environmentName = "dev",
				status = "applied",
				changedFields = {
					{ field = "metadata.annotations.removed", oldValue = "old" },
					{ field = "metadata.labels.removed",      oldValue = "old" },
					{ field = "spec.resources.limits.cpu",    oldValue = "1" },
				},
			},
			{
				resource = "ConfigMap/my-config",
				environmentName = "dev",
				status = "applied",
				changedFields = {
					{ field = "binaryData.removed", oldValue = "b2xk" },
					{ field = "data.kept",          oldValue = "old", newValue = "new" },
					{ field = "data.removed",       oldValue = "old" },
				},
			},
		},
	})
end)

Test.k8s("omitted CPU limit and metadata are removed from the stored Application", function(t)
	t.check("nais.io/v1alpha1", "applications", "dev", team:slug(), "my-app", {
		apiVersion = "nais.io/v1alpha1",
		kind = "Application",
		metadata = {
			name = "my-app",
			namespace = team:slug(),
			labels = { kept = "same" },
		},
		spec = {
			image = "example.com/my-app:v1",
			resources = {
				limits = { memory = "512Mi" },
				requests = { cpu = "100m", memory = "256Mi" },
			},
		},
	})
end)

Test.k8s("omitted data keys and top-level fields are removed from the stored ConfigMap", function(t)
	t.check("v1", "configmaps", "dev", team:slug(), "my-config", {
		apiVersion = "v1",
		kind = "ConfigMap",
		metadata = {
			name = "my-config",
			namespace = team:slug(),
		},
		data = { kept = "new" },
	})
end)

Test.rest("reapplying the replacement has no changes", function(t)
	t.addHeader("x-user-email", user:email())
	t.send("POST", "/api/v1/teams/apply-replace-team/environments/dev/apply", replacement)
	t.check(200, {
		results = {
			{ resource = "Application/my-app",  environmentName = "dev", status = "applied" },
			{ resource = "ConfigMap/my-config", environmentName = "dev", status = "applied" },
		},
	})
end)
