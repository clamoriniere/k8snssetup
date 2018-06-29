pkgs = $(shell go list ./... | grep -v /vendor/)

all: format test install

build:
	go build ./cmd/k8snssetup

install:
	go install github.com/cedriclam/k8snssetup/cmd/k8snssetup

test:
	./hack/test

format:
	go fmt $(pkgs)

dep:
	dep ensure -v

.PHONY: all install test format dep