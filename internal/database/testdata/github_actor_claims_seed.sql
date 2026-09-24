-- Local-only sample rows from the activity-log export; not a migration.
INSERT INTO activity_log_entries (
	created_at, actor, action, resource_type, resource_name, data
)
VALUES
	(
		'2026-09-24T08:06:09.334246Z',
		'github-repo:navikt/klang',
		'CREATED', 'JOB', 'klang-e2e-tests-f84a987c-18ba-435b-b725-6f1149cd0360',
		CONVERT_TO($json${"apiVersion":"nais.io/v1","kind":"Naisjob","changedFields":null,"gitHubActorClaims":{"ref":"refs/heads/main","repository":"navikt/klang","repositoryId":"253449616","runId":"35972939471","runAttempt":"1","actor":"eriksson-daniel","workflow":"Deploy (dev -\u003e e2e -\u003e prod)","eventName":"push","environment":"","jobWorkflowRef":"navikt/klang/.github/workflows/deploy-to-dev.yaml@refs/heads/main"}}$json$, 'UTF8')
	),
	(
		'2026-09-24T08:05:51.090501Z',
		'github-repo:navikt/kaka',
		'UPDATED', 'APP', 'kaka-frontend',
		CONVERT_TO($json${"apiVersion":"nais.io/v1alpha1","kind":"Application","changedFields":[{"field":"spec.image","oldValue":"europe-north1-docker.pkg.dev/nais-management-233d/klage/kaka-frontend:ae6f43278929c27b759a0d49770a69cdc2f34f57","newValue":"europe-north1-docker.pkg.dev/nais-management-233d/klage/kaka-frontend:2621bf4c8f0a0ff891d0b2b5e497939b748e8693"}],"gitHubActorClaims":{"ref":"refs/heads/main","repository":"navikt/kaka","repositoryId":"419700565","runId":"35972893326","runAttempt":"1","actor":"eriksson-daniel","workflow":"Deploy (dev -\u003e e2e -\u003e prod)","eventName":"push","environment":"","jobWorkflowRef":"navikt/kaka/.github/workflows/deploy-to-prod.yaml@refs/heads/main"}}$json$, 'UTF8')
	),
	(
		'2026-06-25T08:11:32.265277Z',
		'github-repo:nais/example',
		'UPDATED', 'ConfigMap', 'example-multi-res',
		CONVERT_TO($json${"apiVersion":"v1","kind":"ConfigMap","changedFields":[{"field":"metadata.annotations.deploy.nais.io/github-sha","oldValue":"c0dcd9d415c316eacb4400329880ecc9a75ec143","newValue":"4b5e7715b6661c51e24f87807bf8ab333ad089c2"},{"field":"metadata.annotations.deploy.nais.io/github-workflow-run-url","oldValue":"https://github.com/nais/example/actions/runs/28156238579","newValue":"https://github.com/nais/example/actions/runs/28155892891"},{"field":"metadata.annotations.kubernetes.io/change-cause","oldValue":"nais deploy: commit c0dcd9d415c316eacb4400329880ecc9a75ec143: https://github.com/nais/example/actions/runs/28156238579","newValue":"nais deploy: commit 4b5e7715b6661c51e24f87807bf8ab333ad089c2: https://github.com/nais/example/actions/runs/28155892891"}],"gitHubActorClaims":{"ref":"refs/heads/main","repository":"nais/example","repositoryId":"1274180294","runId":"28155892891","runAttempt":"1","actor":"frodesundby","workflow":"Deploy app (v3 multi-resource-files)","eventName":"push","environment":"","jobWorkflowRef":"nais/example/.github/workflows/deploy-app-v3-multi-resource-files.yaml@refs/heads/main"}}$json$, 'UTF8')
	)
;
