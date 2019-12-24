.DEFAULT_GOAL := docker-build
.PHONY: docker-build docker-tag docker-push

TAG ?= latest
FROM_TAG ?= latest

docker-build:
ifndef NOBUILD
	make -C ../../.. docker-build-fwbuild-manager
endif
	docker build -t docker.io/mgos/fwbuild-manager:$(TAG) .

docker-tag:
	docker tag docker.io/mgos/fwbuild-manager:$(FROM_TAG) docker.io/mgos/fwbuild-manager:$(TAG)

docker-push:
	docker push docker.io/mgos/fwbuild-manager:$(TAG)
