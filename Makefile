APP_NAME ?= kaptest
DOCKER_BUILD ?= DOCKER_BUILDKIT=1 docker build --progress=plain
GOOS ?= linux
GOARCH ?= amd64

.PHONY: test
test:
	${DOCKER_BUILD} --target test --output . .

.PHONY: lint
lint:
	${DOCKER_BUILD} --target lint .

.PHONY: build
build:
	${DOCKER_BUILD} --build-arg GOOS=$(GOOS) --build-arg GOARCH=$(GOARCH) \
		--build-arg APP_NAME=$(APP_NAME) --target export-binary --output . .

GOCREDITS_VERSION ?= v0.3.1
.PHONY: gocredits
gocredits:
	go install github.com/Songmu/gocredits/cmd/gocredits@${GOCREDITS_VERSION}

.PHONY: credits
credits: gocredits
	gocredits . > CREDITS
