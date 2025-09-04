FROM golang:1.24 AS builder
ARG GOOS
ARG GOARCH
ARG APP_NAME

WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download

# Lint
FROM builder AS lint-internal
COPY --from=golangci/golangci-lint:v1.64 /usr/bin/golangci-lint /usr/bin/golangci-lint
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    golangci-lint run

## Export the lint results only
FROM scratch AS lint
COPY --from=lint-internal /app /

# Test
FROM builder AS test-internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    go test -v ./...

# Test examples
FROM builder AS test-examples
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    go run internal/main.go run ./examples/**/*/kaptest.yaml

## Export the test results only
FROM scratch AS test
COPY --from=test-internal /app /

# Build the binary
FROM builder AS build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOOS=${GOOS:-linux} GOARCH=${GOARCH:-amd64} go build -a -o /${APP_NAME:-kaptest} ./internal/main.go

FROM scratch AS export-binary
ARG APP_NAME
COPY --from=build /${APP_NAME:-kaptest} /

FROM builder AS credits
ARG GOCREDITS_VERSION
RUN set -x && \
    go install github.com/Songmu/gocredits/cmd/gocredits@${GOCREDITS_VERSION}
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    set -x && \
    ls -alh /go/pkg/mod/github.com && \
    gocredits -skip-missing . >/CREDITS

FROM scratch AS export-credits
COPY --from=credits /CREDITS /
