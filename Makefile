TEST_POSTGRES_CONTAINER_NAME = nais-api-postgres-integration-test
TEST_POSTGRES_CONTAINER_PORT = 5666

.PHONY: all

all: generate fmt test check build helm-lint

generate: generate-sql generate-graphql generate-mocks generate-proto

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

generate-proto:
	protoc \
		-I pkg/protoapi/schema/ \
		./pkg/protoapi/schema/*.proto \
		--go_out=. \
		--go-grpc_out=.

build:
	go build -o bin/api ./cmd/api
	go build -o bin/setup_local ./cmd/setup_local

local:
	env bash -c 'source local.env; go run ./cmd/api'

test:
	go test ./...

test-with-cc:
	go test -cover --race ./...

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

setup-local:
	GOOGLE_MANAGEMENT_PROJECT_ID=nais-local-dev go run ./cmd/setup_local -users 40 -teams 10 -owners 2 -members 4 -provision_pub_sub

stop-integration-test-db:
	docker stop $(TEST_POSTGRES_CONTAINER_NAME) || true && docker rm $(TEST_POSTGRES_CONTAINER_NAME) || true

start-integration-test-db: stop-integration-test-db
	docker run -d -e POSTGRES_PASSWORD=postgres --name $(TEST_POSTGRES_CONTAINER_NAME) -p $(TEST_POSTGRES_CONTAINER_PORT):5432 postgres:14-alpine

integration-test: start-integration-test-db
	go test ./... -tags=db_integration_test
