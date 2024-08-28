FROM golang:1.22 AS builder
ARG GOOS
ARG GOARCH
ARG APP_NAME

WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download


# Test
FROM builder AS test-internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    go test -v ./...

## Export the test results only
FROM scratch AS test
COPY --from=test-internal /app /


# Build the binary
FROM builder AS build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOOS=${GOOS:-linux} GOARCH=${GOARCH:-amd64} go build -a -o /${APP_NAME:-kaptest} ./main.go


FROM scratch AS export-binary
ARG APP_NAME
COPY --from=build /${APP_NAME:-kaptest} /
