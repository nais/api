LUA_FORMATTER_VERSION = 1.5.6
BIN_DIR := $(shell pwd)/bin
LUAFMT=$(BIN_DIR)/luafmt-$(LUA_FORMATTER_VERSION)

.PHONY: all local

all: generate fmt test check build helm-lint

generate: generate-sql generate-graphql generate-proto generate-mocks

generate-sql:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc generate -f .configs/sqlc.yaml
	go run github.com/sqlc-dev/sqlc/cmd/sqlc vet -f .configs/sqlc.yaml
	go run mvdan.cc/gofumpt@latest -w ./

generate-graphql:
	go run github.com/99designs/gqlgen generate --config .configs/gqlgen.yaml
	go run ./cmd/gen_complexity
	go run mvdan.cc/gofumpt@latest -w ./internal/graph

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
	go run ./cmd/api

debug:
	dlv debug --headless --listen=:2345 --api-version=2 ./cmd/api

test:
	go test -cover -tags integration_test --race ./... github.com/nais/api/pkg/apiclient/...

unit-test:
	go test --race ./... github.com/nais/api/pkg/apiclient/...

check: staticcheck vulncheck deadcode gosec

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

deadcode:
	go run golang.org/x/tools/cmd/deadcode@latest -test ./...

gosec:
	go run github.com/securego/gosec/v2/cmd/gosec@latest --exclude G404,G101 --exclude-generated -terse ./...
# We've disabled G404 and G101 as they are not relevant for our use case
# G404: Use of weak random number generator (math/rand instead of crypto/rand).
#    We don't use random numbers for security purposes.
# G101: Look for hard coded credentials
#    The check for credentials is a bit weak and triggers on multiple variables just including
#    the word `secret`. We depend on GitHub to find possible credentials in our code.

fmt: prettier install-lua-formatter
	go run mvdan.cc/gofumpt@latest -w ./
	$(LUAFMT)/bin/CodeFormat format -w . --ignores-file ".gitignore" -c ./integration_tests/.editorconfig

prettier:
	npm install
	npx prettier --write .

helm-lint:
	helm lint --strict ./charts

graphql-lint:
	npx eslint --cache

setup-local:
	GOOGLE_MANAGEMENT_PROJECT_ID=nais-local-dev go run ./cmd/setup_local -users 40 -teams 10 -owners 2 -members 4 -provision_pub_sub

integration_test:
	rm -f hack/coverprofile.txt
	go test -coverprofile=hack/coverprofile.txt -coverpkg github.com/nais/api/... -v -tags integration_test --race ./integration_tests
# go test -coverprofile=hack/coverprofile.txt -coverpkg $(shell go list --deps ./cmd/api | grep nais/api/ | grep -Ev 'gengql|/(\w+)/\1sql' | tr '\n' ',' | sed '$$s/,$$//') -v -tags integration_test --race ./integration_tests

integration_test_ui:
	go run ./cmd/tester_run --ui

tester_spec:
	go run ./cmd/tester_spec

LUA_FORMATTER_URL := https://github.com/CppCXY/EmmyLuaCodeStyle/releases/download/$(LUA_FORMATTER_VERSION)
OS := $(shell uname -s)
ARCH := $(shell uname -m)

ifeq ($(OS), Darwin)
  ifeq ($(ARCH), x86_64)
    LUA_FORMATTER_FILE := darwin-x64
  else
    ifeq ($(ARCH), arm64)
      LUA_FORMATTER_FILE := darwin-arm64
    else
      $(error Unsupported architecture: $(ARCH) on macOS)
    endif
  endif
else ifeq ($(OS), Linux)
  ifeq ($(ARCH), x86_64)
    LUA_FORMATTER_FILE := linux-x64
  else
    ifeq ($(ARCH), aarch64)
      LUA_FORMATTER_FILE := linux-aarch64
    else
      $(error Unsupported architecture: $(ARCH) on Linux)
    endif
  endif
else
  $(error Unsupported OS: $(OS))
endif

install-lua-formatter: $(LUAFMT)
$(LUAFMT):
	@mkdir -p $(LUAFMT)
	@curl -L $(LUA_FORMATTER_URL)/$(LUA_FORMATTER_FILE).tar.gz -o /tmp/luafmt.tar.gz
	@tar -xzf /tmp/luafmt.tar.gz -C $(LUAFMT)
	@rm /tmp/luafmt.tar.gz
	@mv $(LUAFMT)/$(LUA_FORMATTER_FILE)/* $(LUAFMT)/
	@rmdir $(LUAFMT)/$(LUA_FORMATTER_FILE)
