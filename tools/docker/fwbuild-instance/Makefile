.PHONY: docker-build docker-tag docker-push

TAG ?= latest
FROM_TAG ?= latest

all: docker-build

docker-build:
	$(CURDIR)/../golang/dogobuild \
	  docker.cesanta.com/fwbuild-instance \
	  cesanta.com/fwbuild/instance \
	  Dockerfile \
	  docker.cesanta.com/gobuild

docker-tag:
	docker tag docker.cesanta.com/fwbuild-instance:$(FROM_TAG) docker.cesanta.com/fwbuild-instance:$(TAG)

docker-push:
	docker push docker.cesanta.com/fwbuild-instance:$(TAG)
