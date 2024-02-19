# Migrate from console-backend and teams-backend to api

## Download required service accounts

To connect and authenticate to the correct databases, you'll need to download the service accounts for both apps.

```bash
export PROJECT_ID=nais-management-233d
gcloud auth login
mkdir -p scratchpad
echo '*' > scratchpad/.gitignore
cd scratchpad
gcloud iam service-accounts keys create console-backend.json --iam-account console-backend@$PROJECT_ID.iam.gserviceaccount.com
gcloud iam service-accounts keys create teams-backend.json --iam-account console@$PROJECT_ID.iam.gserviceaccount.com
cd ..

# Create a list of all database instances to know the random part of the instance name
gcloud sql instances list --project $PROJECT_ID --format="value(name)"

# Start a SQL proxy to the api database.
cloud_sql_proxy $PROJECT_ID:europe-north1:nais-api-[random-part]
```

## To migrate Teams-backend

You'll need both apps GCP service accounts to do the following:

Open a SQL proxy to both teams-backend and the new api database:

```bash
cloud_sql_proxy --credentials-file ./scratchpad/api.json -i $PROJECT_ID:europe-north1:console-backend-[random-part]
```

Update the consts in `cmd/migrate/main.go` to point to the correct databases.

Run the migration script:

```bash
go run ./cmd/migrate
```

## To migrate console-backend

You'll need both apps GCP service accounts to do the following:

Open a SQL proxy to both console-backend and the new api database:

```bash
cloud_sql_proxy --credentials-file ~/Downloads/nais-management-233d-a47334b216f8.json -i nais-management-233d:europe-north1:console-backend-e36c4211-thomas
```

Dump the console-backend database:

```bash
pg_dump --host 127.0.0.1 --username console-backend@nais-management-233d.iam -Ox -n public -t resource_utilization_metrics --data-only console_backend > console-backend.sql
```

(This will dump data from the `resource_utilization_metrics` table)

Update column names in `console-backend.sql` to match the new schema.

```bash
sed -i 's/COPY public\.resource_utilization_metrics.*/COPY public.resource_utilization_metrics (id, "timestamp", environment, team_slug, app, resource_type, usage, request) FROM stdin;/g' console-backend.sql
```

Run psql:

```bash
psql --host 127.0.0.1 --port 3002 --username console-backend@nais-management-233d.iam -f console-backend.sql api
```

## Delete service account keys

```bash
cd scratchpad
gcloud iam service-accounts keys delete $(jq -r '.private_key_id' ./scratchpad/api.json) --iam-account nais-api@$PROJECT_ID.iam.gserviceaccount.com && rm ./scratchpad/api.json
gcloud iam service-accounts keys delete $(jq -r '.private_key_id' ./scratchpad/console-backend.json) --iam-account console-backend@$PROJECT_ID.iam.gserviceaccount.com && rm ./scratchpad/console-backend.json
gcloud iam service-accounts keys delete $(jq -r '.private_key_id' ./scratchpad/teams-backend.json) --iam-account console@$PROJECT_ID.iam.gserviceaccount.com && rm ./scratchpad/teams-backend.json
```
