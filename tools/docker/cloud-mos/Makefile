TAG ?= latest
FROM_TAG ?= latest

all: docker-build

docker-build:
	$(CURDIR)/../golang/dogobuild \
	  docker.io/mgos/mos \
	  cesanta.com/mos \
	  Dockerfile \
	  docker.io/mgos/gobuild \
	  -v -tags noflash

docker-tag:
	docker tag docker.io/mgos/mos:$(FROM_TAG) docker.io/mgos/mos:$(TAG)

docker-push:
	docker push docker.io/mgos/mos:$(TAG)
