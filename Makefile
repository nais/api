.PHONY: all local

all:
	mise run all

generate:
	mise run generate

generate-sql:
	mise run -j 1 generate:sql ::: fmt:go

generate-graphql:
	mise run -j 1 generate:graphql ::: fmt:go

generate-mocks:
	mise run -j 1 generate:mocks ::: fmt:go

generate-proto:
	mise run -j 1 generate:proto ::: fmt:go

build:
	mise run build

local:
	mise run local

debug:
	mise run local:debug

test:
	mise run test

unit-test:
	mise run test:unit

check:
	mise run check

staticcheck:
	mise run check:staticcheck

vulncheck:
	mise run check:vulncheck

deadcode:
	mise run check:deadcode

gosec:
	mise run check:gosec

fmt:
	mise run fmt

prettier:
	mise run fmt:prettier

helm-lint:
	mise run check:helm-lint

graphql-lint:
	mise run check:graphql-lint

setup-local:
	mise run local:setup

integration_test:
	mise run test --coverage

integration_test_ui:
	mise run test:ui

tester_spec:
	mise run -j 1 generate:tester_spec ::: fmt:prettier

lint:
	mise run check:vulncheck ::: check:staticcheck
	mise run generate fmt

