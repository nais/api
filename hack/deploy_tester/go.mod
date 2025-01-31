module github.com/nais/api/hack/deploy_tester

go 1.23.1

replace github.com/nais/api/pkg/apiclient => ../../pkg/apiclient

require (
	github.com/nais/api/pkg/apiclient v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.65.0
	k8s.io/utils v0.0.0-20241210054802-24370beab758
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
