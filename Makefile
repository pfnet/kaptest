APP_NAME ?= kaptest
DOCKER_BUILD ?= DOCKER_BUILDKIT=1 docker build --progress=plain
GOOS ?= linux
GOARCH ?= amd64

.PHONY: test
test:
	${DOCKER_BUILD} --target test .

.PHONY: test-examples
test-examples:
	${DOCKER_BUILD} --target test-examples .


.PHONY: lint
lint:
	${DOCKER_BUILD} --target lint .

.PHONY: build
build:
	${DOCKER_BUILD} --build-arg GOOS=$(GOOS) --build-arg GOARCH=$(GOARCH) \
		--build-arg APP_NAME=$(APP_NAME) --target export-binary --output . .

GOCREDITS_VERSION ?= v0.4.0
.PHONY: credits
credits: go.sum
	$(DOCKER_BUILD) --build-arg GOCREDITS_VERSION=$(GOCREDITS_VERSION) --target export-credits --output . .
