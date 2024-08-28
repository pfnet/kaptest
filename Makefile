APP_NAME ?= kaptest
DOCKER_BUILD ?= DOCKER_BUILDKIT=1 docker build --progress=plain
GOOS ?= linux
GOARCH ?= amd64

.PHONY: test
test:
	${DOCKER_BUILD} --target test --output . .

.PHONY: build
build:
	${DOCKER_BUILD} --build-arg GOOS=$(GOOS) --build-arg GOARCH=$(GOARCH) \
		--build-arg APP_NAME=$(APP_NAME) --target export-binary --output . .

.PHONY: cli-test
cli-test:
	go run internal/cmd/main.go --verbose examples/cli/complicated/manifest.yaml
