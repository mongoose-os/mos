all: build-all

UBUNTU_DISTRS := xenial bionic focal impish jammy

build-all: $(foreach u,$(UBUNTU_DISTRS),docker-build-ubuntu-golang-$(u)) docker-build-ubuntu32-golang-bionic docker-build-golang-mingw

push-all: $(foreach u,$(UBUNTU_DISTRS),docker-push-ubuntu-golang-$(u)) docker-push-ubuntu32-golang-bionic docker-push-golang-mingw

pull-all: $(foreach u,$(UBUNTU_DISTRS),docker-pull-ubuntu-golang-$(u)) docker-pull-ubuntu32-golang-bionic docker-pull-golang-mingw

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
