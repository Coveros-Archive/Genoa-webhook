VERSION ?= 0.1.0
# Image URL to use all building/pushing image targets
IMG ?= coveros/genoa-webhook:${VERSION}


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build-binary

# Run tests
test: fmt vet
	go test -v ./... -coverprofile cover.out

# Build webhook binary
build-binary: fmt vet
	go build -o bin/genoa-webhook main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
	go run ./main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

mody-tidy:
	go mod tidy

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

local-build: mody-tidy vet fmt docker-build

build: local-build docker-push

deploy-chart:
	kubectl create ns genoa || true
	helm upgrade genoa-webhook charts/genoa-webhook --install --namespace=genoa --set deployments.image.tag=${VERSION}