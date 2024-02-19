# Migrate from console-backend and teams-backend to api

## Download required service accounts

To connect and authenticate to the correct databases, you'll need to download the service accounts for both apps.

```bash
export PROJECT_ID=MANAGEMENT_ID
gcloud auth login
mkdir -p scratchpad
echo '*' > scratchpad/.gitignore
gcloud iam service-accounts keys create scratchpad/console-backend.json --iam-account console-backend@$PROJECT_ID.iam.gserviceaccount.com
gcloud iam service-accounts keys create scratchpad/teams-backend.json --iam-account console@$PROJECT_ID.iam.gserviceaccount.com

# Create a list of all database instances to know the random part of the instance name
gcloud sql instances list --project $PROJECT_ID --format="value(name)"

# Start a SQL proxy to the api database.
cloud_sql_proxy "${PROJECT_ID}:europe-north1:$(gcloud sql instances list --project $PROJECT_ID --format="value(name)" | grep nais-api)"
```

## To migrate both backends to new api

You'll need both apps GCP service accounts to do the following:

Open a SQL proxy to both teams-backend (database `console-[random-part]`) and console-backend database (database `console-backend-[random-part]`):

```bash
cloud_sql_proxy --port 6000 --credentials-file ./scratchpad/teams-backend.json -i "${PROJECT_ID}:europe-north1:$(gcloud sql instances list --project $PROJECT_ID --format="value(name)" | grep console | grep -v backend)"
cloud_sql_proxy --port 7000 --credentials-file ./scratchpad/console-backend.json -i "${PROJECT_ID}:europe-north1:$(gcloud sql instances list --project $PROJECT_ID --format="value(name)" | grep console-backend)"
```

Update the consts in `cmd/migrate/main.go` to point to the correct databases, as well as the list of environments.

Run the migration script:

```bash
go run ./cmd/migrate
```

Close the SQL proxies.

## Delete service account keys

```bash
gcloud iam service-accounts keys delete $(jq -r '.private_key_id' ./scratchpad/console-backend.json) --iam-account console-backend@$PROJECT_ID.iam.gserviceaccount.com && rm ./scratchpad/console-backend.json
gcloud iam service-accounts keys delete $(jq -r '.private_key_id' ./scratchpad/teams-backend.json) --iam-account console@$PROJECT_ID.iam.gserviceaccount.com && rm ./scratchpad/teams-backend.json
```
