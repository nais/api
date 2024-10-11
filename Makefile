TEST_POSTGRES_CONTAINER_NAME = nais-api-postgres-integration-test
TEST_POSTGRES_CONTAINER_PORT = 5666

.PHONY: all

all: generate fmt test check build helm-lint

generate: generate-sql generate-graphql generate-proto generate-mocks

generate-sql:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc generate -f .configs/sqlc.yaml
	go run github.com/sqlc-dev/sqlc/cmd/sqlc vet -f .configs/sqlc.yaml
	go run mvdan.cc/gofumpt@latest -w ./internal/database/gensql

generate-sql-v1:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc generate -f .configs/sqlc-v1.yaml
	go run github.com/sqlc-dev/sqlc/cmd/sqlc vet -f .configs/sqlc-v1.yaml
	go run mvdan.cc/gofumpt@latest -w ./

generate-graphql:
	go run github.com/99designs/gqlgen generate --config .configs/gqlgen.yaml
	go run mvdan.cc/gofumpt@latest -w ./internal/graph

generate-graphql-v1:
	go run github.com/99designs/gqlgen generate --config .configs/gqlgen-v1.yaml
	go run mvdan.cc/gofumpt@latest -w ./internal/v1/graphv1

generate-mocks:
	find internal pkg -type f -name "mock_*.go" -delete
	go run github.com/vektra/mockery/v2 --config ./.configs/mockery.yaml
	find internal pkg -type f -name "mock_*.go" -exec go run mvdan.cc/gofumpt@latest -w {} \;

generate-proto:
	protoc \
		-I pkg/apiclient/protoapi/schema/ \
		./pkg/apiclient/protoapi/schema/*.proto \
		--go_out=. \
		--go-grpc_out=.

build:
	go build -o bin/api ./cmd/api
	go build -o bin/setup_local ./cmd/setup_local

local:
	env bash -c 'source local.env; go run ./cmd/api'

debug:
	env bash -c 'source local.env; dlv debug --headless --listen=:2345 --api-version=2 ./cmd/api'

test:
	go test ./... github.com/nais/api/pkg/apiclient/...

test-with-cc:
	go test -cover --race ./... github.com/nais/api/pkg/apiclient/...

check: staticcheck vulncheck deadcode

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

deadcode:
	go run golang.org/x/tools/cmd/deadcode@latest -test ./...

fmt: prettier
	go run mvdan.cc/gofumpt@latest -w ./

prettier:
	npm install
	npx prettier --write .

helm-lint:
	helm lint --strict ./charts

setup-local:
	GOOGLE_MANAGEMENT_PROJECT_ID=nais-local-dev go run ./cmd/setup_local -users 40 -teams 10 -owners 2 -members 4 -provision_pub_sub

stop-integration-test-db:
	docker stop $(TEST_POSTGRES_CONTAINER_NAME) || true && docker rm $(TEST_POSTGRES_CONTAINER_NAME) || true

start-integration-test-db: stop-integration-test-db
	docker run -d -e POSTGRES_PASSWORD=postgres --name $(TEST_POSTGRES_CONTAINER_NAME) -p $(TEST_POSTGRES_CONTAINER_PORT):5432 postgres:14-alpine

integration_test:
	rm -f hack/coverprofile.txt
	go test -coverprofile=hack/coverprofile.txt -coverpkg github.com/nais/api/... -v -tags integration_test --race ./integration_tests

tester_spec:
	go run ./cmd/tester_spec