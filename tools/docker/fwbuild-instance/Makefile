.PHONY: docker-build docker-tag docker-push

TAG ?= latest
FROM_TAG ?= latest

all: docker-build

docker-build:
	$(CURDIR)/../golang/dogobuild \
	  docker.io/mgos/fwbuild-instance \
	  cesanta.com/fwbuild/instance \
	  Dockerfile \
	  docker.io/mgos/gobuild

docker-tag:
	docker tag docker.io/mgos/fwbuild-instance:$(FROM_TAG) docker.io/mgos/fwbuild-instance:$(TAG)

docker-push:
	docker push docker.io/mgos/fwbuild-instance:$(TAG)
