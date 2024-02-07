ARG GO_VERSION=""
FROM golang:${GO_VERSION}alpine as builder
WORKDIR /src
COPY go.* /src/
RUN go mod download
COPY . /src
RUN go build -o bin/api ./cmd/api

FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /src/bin/api /app/api
ENTRYPOINT ["/app/api"]
