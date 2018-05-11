.PHONY: docker-build docker-tag docker-push

TAG ?= latest
FROM_TAG ?= latest

all: docker-build

generate:
	go generate cesanta.com/fwbuild/manager/...

docker-build: generate
	$(CURDIR)/../golang/dogobuild \
	  docker.cesanta.com/fwbuild-manager \
	  cesanta.com/fwbuild/manager \
	  Dockerfile \
	  docker.cesanta.com/gobuild

docker-tag:
	docker tag docker.cesanta.com/fwbuild-manager:$(FROM_TAG) docker.cesanta.com/fwbuild-manager:$(TAG)

docker-push:
	docker push docker.cesanta.com/fwbuild-manager:$(TAG)
