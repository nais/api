.PHONY: all

all: generate fmt test check api helm-lint

generate: generate-sql generate-graphql generate-mocks

generate-sql:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc generate -f .configs/sqlc.yaml
	go run github.com/sqlc-dev/sqlc/cmd/sqlc vet -f .configs/sqlc.yaml
	go run mvdan.cc/gofumpt@latest -w ./internal/database/gensql

generate-graphql:
	go run github.com/99designs/gqlgen generate --config .configs/gqlgen.yaml
	go run mvdan.cc/gofumpt@latest -w ./internal/graph

generate-mocks:
	go run github.com/vektra/mockery/v2 --config ./.configs/mockery.yaml
	find internal -type f -name "mock_*.go" -exec go run mvdan.cc/gofumpt@latest -w {} \;

setup:
	gcloud secrets versions access latest --secret=api-kubeconfig --project aura-dev-d9f5 > kubeconfig

api:
	go build -o bin/api ./cmd/api/main.go

portforward-hookd:
	kubectl port-forward -n nais-system --context nav-management-v2 svc/hookd 8282:80

portforward-teams:
	kubectl port-forward -n nais-system --context nav-management-v2 svc/api 8181:80

local-nav:
	DEPENDENCYTRACK_ENDPOINT="https://dependencytrack-backend.nav.cloud.nais.io" \
	DEPENDENCYTRACK_FRONTEND="https://salsa.nav.cloud.nais.io" \
	DEPENDENCYTRACK_USERNAME="todo" \
	DEPENDENCYTRACK_PASSWORD="todo" \
	BIGQUERY_PROJECTID="nais-io" \
	HOOKD_ENDPOINT="http://localhost:8282" \
	HOOKD_PSK="$(shell kubectl get secret api --context nav-management-v2 -n nais-system -ojsonpath='{.data.HOOKD_PSK}' | base64 --decode)" \
	KUBERNETES_CLUSTERS="dev-gcp,prod-gcp" \
	KUBERNETES_CLUSTERS_STATIC="dev-fss|apiserver.dev-fss.nais.io|$(shell kubectl get secret --context nav-dev-fss --namespace nais-system api -ojsonpath='{ .data.token }' | base64 --decode)" \
	LISTEN_ADDRESS="127.0.0.1:4242" \
	LOG_FORMAT="text" \
	LOG_LEVEL="debug" \
	RUN_AS_USER="johnny.horvi@nav.no" \
	API_ENDPOINT="http://localhost:8181/query" \
	API_TOKEN="$(shell kubectl get secret api --context nav-management-v2 -n nais-system -ojsonpath='{.data.API_TOKEN}' | base64 --decode)" \
	TENANT="nav" \
	go run ./cmd/api/main.go

local:
	HOOKD_ENDPOINT="http://hookd.local.nais.io" \
	KUBERNETES_CLUSTERS="ci,dev" \
	LISTEN_ADDRESS="127.0.0.1:4242" \
	LOG_FORMAT="text" \
	LOG_LEVEL="debug" \
	RUN_AS_USER="dev.usersen@nais.io" \
	API_ENDPOINT="http://teams.local.nais.io:3000/query" \
	go run ./cmd/api/main.go

test:
	go test ./... -v

check: staticcheck vulncheck deadcode

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

deadcode:
	go run golang.org/x/tools/cmd/deadcode@latest -test ./...

fmt:
	go run mvdan.cc/gofumpt@latest -w ./

helm-lint:
	helm lint --strict ./charts
