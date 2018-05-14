all: build-all

build-all: docker-build-ubuntu-golang-xenial docker-build-ubuntu-golang-artful docker-build-ubuntu-golang-bionic \
           docker-build-ubuntu32-golang-bionic \
           docker-build-gobuild docker-build-gobuild32 docker-build-gobuild-mingw

push-all:  docker-push-ubuntu-golang-xenial docker-push-ubuntu-golang-artful docker-push-ubuntu-golang-bionic \
           docker-push-ubuntu32-golang-bionic \
           docker-push-gobuild docker-push-gobuild32 docker-push-gobuild-mingw

pull-all:  docker-pull-ubuntu-golang-xenial docker-pull-ubuntu-golang-artful docker-pull-ubuntu-golang-bionic \
           docker-pull-ubuntu32-golang-bionic \
           docker-pull-gobuild docker-pull-gobuild32 docker-pull-gobuild-mingw

docker-build-ubuntu-golang-%: Dockerfile.ubuntu-golang
	docker build -t docker.cesanta.com/ubuntu-golang:$* -f Dockerfile.ubuntu-golang --build-arg base=ubuntu:$* .

docker-build-ubuntu32-golang-%: Dockerfile.ubuntu-golang
	docker build -t docker.cesanta.com/ubuntu32-golang:$* -f Dockerfile.ubuntu-golang --build-arg base=cesanta/ubuntu32:$* .

docker-build-%: Dockerfile.%
	docker build -t docker.cesanta.com/$* -f Dockerfile.$* .

docker-push-ubuntu-golang-%:
	docker push docker.cesanta.com/ubuntu-golang:$*

docker-push-ubuntu32-golang-%:
	docker push docker.cesanta.com/ubuntu32-golang:$*

docker-push-%:
	docker push docker.cesanta.com/$*

docker-pull-ubuntu-golang-%:
	docker pull docker.cesanta.com/ubuntu-golang:$*

docker-pull-ubuntu32-golang-%:
	docker pull docker.cesanta.com/ubuntu32-golang:$*

docker-pull-%:
	docker pull docker.cesanta.com/$*
