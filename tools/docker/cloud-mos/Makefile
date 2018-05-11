TAG ?= latest
FROM_TAG ?= latest

all: docker-build

docker-build:
	$(CURDIR)/../golang/dogobuild \
	  docker.cesanta.com/mos \
	  cesanta.com/mos \
	  Dockerfile \
	  docker.cesanta.com/gobuild \
	  -v -tags noflash

docker-tag:
	docker tag docker.cesanta.com/mos:$(FROM_TAG) docker.cesanta.com/mos:$(TAG)

docker-push:
	docker push docker.cesanta.com/mos:$(TAG)
