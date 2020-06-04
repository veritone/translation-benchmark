GIT_COMMIT := $(shell git rev-parse HEAD)
OSX_BUILD_FLAGS := -s
GOBIN := $(GOPATH)/bin
LIST_NO_VENDOR := $(go list ./... | grep -v /vendor/)
BINARY_NAME := benchmark-engines-rt
ENGINE_NAME := benchmark-engines-rt-v-3-f
ORG_ID := 7682
GITHUB_ACCESS_TOKEN := a22782aa1dea61de3bdface5eb172f28d5ad35a3
# PAYLOAD_FILE := ./payload.json

default: check fmt deps test linux

dev: osx
osx: check fmt deps build

go-build-all: check fmt deps linux

build: linux build-docker

build-sclite:
	docker build -t sclite -f Dockerfile.sctk .

build-go:
	go build -ldflags "$(OSX_BUILD_FLAGS)" -a -o $(BINARY_NAME) .

gen-build-manifest:
	sh ./gen-build-manifest.sh

linux:
	# Build project for linux
	env GOOS=linux GOARCH=amd64 go build -a -o $(BINARY_NAME) .

build.static:
	# Build statically linked binary
	go build -ldflags "$(STATIC_BUILD_FLAGS)" -a -o $(BINARY_NAME) .

check:
	# Only continue if go is installed
	go version || ( echo "Go not installed, exiting"; exit 1 )

test:
	# Run all tests, with coverage (excluding vendored packages)
	go test -coverprofile cp.out

inspect-coverage:
	go tool cover -html=cp.out

clean:
	go clean -i
	rm -rf ./vendor/*/
	rm -f $(BINARY_NAME)

deps:
	# Install or update govend
	go get -u github.com/govend/govend
	# Fetch vendored dependencies
	$(GOBIN)/govend -v

fmt:
	# Format all Go source files (excluding vendored packages)
	go fmt $(LIST_NO_VENDOR)

generate-deps:
	# Generate vendor.yml
	govend -v -l
	git checkout vendor/.gitignore

build-engine-template:
	go get -u github.com/veritone/src-training-workflow/engine-template
	make -C $(GOPATH)/src/github.com/veritone/src-training-workflow/engine-template build-ubuntu

build-docker: build-sclite
	docker build -t $(BINARY_NAME) --build-arg GITHUB_ACCESS_TOKEN=$(GITHUB_ACCESS_TOKEN) .

build-docker-use-engine-template-without-test: gen-build-manifest build-sclite build-engine-template
	docker build -t $(BINARY_NAME) --build-arg GITHUB_ACCESS_TOKEN=$(GITHUB_ACCESS_TOKEN) .

build-docker-without-test: gen-build-manifest build-sclite
	docker build -t $(BINARY_NAME) --build-arg GITHUB_ACCESS_TOKEN=$(GITHUB_ACCESS_TOKEN) .

run-bash:
	docker run -it --entrypoint="/bin/bash" -e PAYLOAD_FILE=$(PAYLOAD_FILE) $(BINARY_NAME)

run-docker-go:
	docker run -it --entrypoint="/app/benchmark-engines-rt" -e PAYLOAD_FILE=$(PAYLOAD_FILE) $(BINARY_NAME)

.PHONY: start-kafka
start-kafka:
	docker-compose up -d
	cp payload.json /tmp/qtest-payload.json

.PHONY: run-docker
run-docker:
	docker run -it -e PAYLOAD_FILE=$(PAYLOAD_FILE) $(BINARY_NAME)

# run-docker: start-kafka
# 	docker run --network=host -e DEVELOPMENT=true -v /tmp:/tmp -e PAYLOAD_FILE=/tmp/qtest-payload.json $(BINARY_NAME)

.PHONY: docker-shell
docker-shell: start-kafka
	docker run --network=host -it --entrypoint=/bin/bash -v /tmp:/tmp -e PAYLOAD_FILE=/tmp/qtest-payload.json $(BINARY_NAME)

.PHONY: push-dev
push-dev:
	docker tag $(BINARY_NAME) docker.aws-dev.veritone.com/$(ORG_ID)/$(ENGINE_NAME)
	docker push docker.aws-dev.veritone.com/$(ORG_ID)/$(ENGINE_NAME)

.PHONY: push-stage
push-stage:
	docker tag $(BINARY_NAME) docker.stage.veritone.com/$(ORG_ID)/$(ENGINE_NAME)
	docker push docker.stage.veritone.com/$(ORG_ID)/$(ENGINE_NAME)

.PHONY: push-prod
push-prod:
	docker tag $(BINARY_NAME) docker.veritone.com/$(ORG_ID)/$(ENGINE_NAME)
	docker push docker.veritone.com/$(ORG_ID)/$(ENGINE_NAME)

.PHONY: push-uk-prod
push-uk-prod:
	docker tag $(BINARY_NAME) docker.uk.veritone.com/1/$(ENGINE_NAME)
	docker push docker.uk.veritone.com/1/$(ENGINE_NAME)
