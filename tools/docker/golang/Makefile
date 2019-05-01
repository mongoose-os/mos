all: build-all

pull-all:  docker-pull-ubuntu-golang-xenial docker-pull-ubuntu-golang-bionic docker-pull-ubuntu-golang-cosmic docker-pull-ubuntu-golang-disco \
           docker-pull-gobuild docker-pull-gobuild32 docker-pull-gobuild-mingw

docker-build-ubuntu-golang-%: Dockerfile.ubuntu-golang
	docker build -t docker.io/mgos/ubuntu-golang:$* -f Dockerfile.ubuntu-golang --build-arg base=ubuntu:$* .

docker-build-ubuntu32-golang-%: Dockerfile.ubuntu-golang
	docker build -t docker.io/mgos/ubuntu32-golang:$* -f Dockerfile.ubuntu-golang --build-arg base=docker.io/mgos/ubuntu32:$* .

docker-build-%: Dockerfile.%
	docker build -t docker.io/mgos/$* -f Dockerfile.$* .

docker-push-ubuntu-golang-%:
	docker push docker.io/mgos/ubuntu-golang:$*

docker-push-ubuntu32-golang-%:
	docker push docker.io/mgos/ubuntu32-golang:$*

docker-push-%:
	docker push docker.io/mgos/$*

docker-pull-ubuntu-golang-%:
	docker pull docker.io/mgos/ubuntu-golang:$*

docker-pull-ubuntu32-golang-%:
	docker pull docker.io/mgos/ubuntu32-golang:$*

docker-pull-%:
	docker pull docker.io/mgos/$*
