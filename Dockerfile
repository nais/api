ARG GO_VERSION=""
FROM golang:${GO_VERSION}alpine AS builder
WORKDIR /src
RUN go env -w GOMODCACHE=/root/.cache/go-build
COPY go.* /src/
COPY pkg/apiclient/go.* /src/pkg/apiclient/
RUN --mount=type=cache,target=/root/.cache/go-build go mod download
COPY internal /src/internal
COPY pkg /src/pkg
COPY cmd /src/cmd
RUN --mount=type=cache,target=/root/.cache/go-build go build -o bin/api ./cmd/api

FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /src/bin/api /app/api
ENTRYPOINT ["/app/api"]
