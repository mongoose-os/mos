all: build-all

build-all: docker-build-ubuntu-golang-xenial docker-build-ubuntu-golang-artful docker-build-ubuntu-golang-bionic

push-all:  docker-push-ubuntu-golang-xenial docker-push-ubuntu-golang-artful docker-push-ubuntu-golang-bionic

#docker-push-ubuntu-golang-xenial docker-push-ubuntu-golang-artful docker-push-ubuntu-golang-bionic

docker-build-ubuntu-golang-%:
	docker build -t docker.cesanta.com/ubuntu-golang:$* -f Dockerfile.ubuntu-golang --build-arg base=ubuntu:$* .

docker-push-ubuntu-golang-%:
	docker push docker.cesanta.com/ubuntu-golang:$*
