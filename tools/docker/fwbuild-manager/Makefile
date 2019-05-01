.PHONY: docker-build docker-tag docker-push

TAG ?= latest
FROM_TAG ?= latest

all: docker-build

generate:
	go generate github.com/mongoose-os/mos/fwbuild/manager/...

docker-build: generate
	$(CURDIR)/../golang/dogobuild \
	  docker.io/mgos/fwbuild-manager \
	  github.com/mongoose-os/mos/fwbuild/manager \
	  Dockerfile \
	  docker.io/mgos/gobuild

docker-tag:
	docker tag docker.io/mgos/fwbuild-manager:$(FROM_TAG) docker.io/mgos/fwbuild-manager:$(TAG)

docker-push:
	docker push docker.io/mgos/fwbuild-manager:$(TAG)
