.DEFAULT_GOAL := docker-build
.PHONY: docker-build docker-tag docker-push

TAG ?= latest
FROM_TAG ?= latest

docker-build:
ifndef NOBUILD
	make -C ../../.. docker-build-mos
endif
	docker build -t docker.io/mgos/mos:$(TAG) .

docker-tag:
	docker tag docker.io/mgos/mos:$(FROM_TAG) docker.io/mgos/mos:$(TAG)

docker-push:
	docker push docker.io/mgos/mos:$(TAG)
