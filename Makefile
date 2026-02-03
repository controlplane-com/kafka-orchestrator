REGISTRY   = ghcr.io
COMPONENT  = kafka-orchestrator
IMAGE      = controlplane-com/$(COMPONENT)
COMPONENT_IMAGE_NAME ?= $(shell echo "$(COMPONENT)" | tr '[:lower:]' '[:upper:]' | tr '-' '_')_IMAGE
TAG ?= latest

VPREFIX           = gitlab.com/controlplane/controlplane/$(COMPONENT)/pkg/about
PROJECT_EPOCH     = $(shell bash -c 'echo $$(( $$(date -u +%s) / 86400 - 18178 ))' )
PROJECT_VERSION   = $(shell git rev-parse --short HEAD)
PROJECT_TIMESTAMP = $(shell date -u +%y%m%d-%H%M%S)

DOCKER_BUILD_ARGS ?=
ENVIRONMENT_NAME  ?= test
LOCATION          ?= default
HOST_UID := $(shell id -u)
HOST_GID := $(shell id -g)

test:
	DOCKER_CONTENT_TRUST="" docker build --platform=linux/amd64 -t \
		$(REGISTRY)/$(IMAGE):tester \
		--build-arg COMPONENT=$(COMPONENT) \
		--build-arg UID=$(HOST_UID) \
		--build-arg GID=$(HOST_GID) \
		--target tester \
		-f Dockerfile .
	docker run --dns 8.8.8.8 --dns 8.8.4.4 --entrypoint /bin/bash $(REGISTRY)/$(IMAGE):tester -c "cd /home/nonroot/service && go test -v -cover ./..."

push-image-arm64:
	DOCKER_CONTENT_TRUST="" docker buildx build -t \
		$(REGISTRY)/$(IMAGE):$(TAG)-arm64 \
		--platform linux/arm64 \
		--build-arg COMPONENT=$(COMPONENT) \
		--build-arg PROJECT_EPOCH=$(PROJECT_EPOCH) \
		--build-arg PROJECT_VERSION=$(PROJECT_VERSION) \
		--build-arg PROJECT_TIMESTAMP=$(PROJECT_TIMESTAMP) \
		--build-arg PROJECT_BUILD=$${CI_PIPELINE_ID:-$${GITHUB_RUN_ID:-0}} \
		--build-arg VPREFIX=$(VPREFIX) \
		--target runner \
		--push \
		-f Dockerfile .

push-image-amd64:
	DOCKER_CONTENT_TRUST="" docker buildx build -t \
		$(REGISTRY)/$(IMAGE):$(TAG)-amd64 \
		--platform linux/amd64 \
		--build-arg COMPONENT=$(COMPONENT) \
		--build-arg PROJECT_EPOCH=$(PROJECT_EPOCH) \
		--build-arg PROJECT_VERSION=$(PROJECT_VERSION) \
		--build-arg PROJECT_TIMESTAMP=$(PROJECT_TIMESTAMP) \
		--build-arg PROJECT_BUILD=$${CI_PIPELINE_ID:-$${GITHUB_RUN_ID:-0}} \
		--build-arg VPREFIX=$(VPREFIX) \
		--target runner \
		--push \
		-f Dockerfile .

push-image:
	docker buildx imagetools create \
		--tag $(REGISTRY)/$(IMAGE):$(TAG) \
		$(REGISTRY)/$(IMAGE):$(TAG)-amd64 \
		$(REGISTRY)/$(IMAGE):$(TAG)-arm64

push-hack-image: push-image-amd64
	docker pull $(REGISTRY)/$(IMAGE):$(TAG)-amd64
	docker tag $(REGISTRY)/$(IMAGE):$(TAG)-amd64 $(REGISTRY)/$(IMAGE):$(TAG)
	docker push $(REGISTRY)/$(IMAGE):$(TAG)

trivy-image:
	trivy image $(REGISTRY)/$(IMAGE):$(TAG) --ignore-unfixed --severity HIGH,CRITICAL ${TRIVY_IMAGE_OPTS}

trivy-fs:
	trivy fs . --ignore-unfixed --severity HIGH,CRITICAL --security-checks vuln ${TRIVY_FS_OPTS}
