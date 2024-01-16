.PHONY: all

all: generate fmt test check api helm-lint

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

api:
	go build -o bin/api ./cmd/api

local:
	KUBERNETES_CLUSTERS="superprod,dev" \
	LOG_FORMAT="text" \
	LOG_LEVEL="debug" \
	WITH_FAKE_CLIENTS="true" \
	go run ./cmd/api

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

seed:
	go run cmd/database-seeder/main.go -users 1000 -teams 100 -owners 2 -members 10
