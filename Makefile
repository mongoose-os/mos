.DEFAULT_GOAL := mos
.PHONY: all build clean clean-tools clean-version deploy-fwbuild deploy-mos-binary deps downloads fwbuild-instance fwbuild-manager generate install linux mac mos version win

TAG ?= latest

GOBUILD_TAGS ?= ''
GOBUILD_LDFLAGS ?= ''
GOBUILD_GOOS ?=
GOBUILD_GOARCH ?=
GOBUILD_CC ?=
GOBUILD_CXX ?=

REPO := $(realpath .)
GOBIN ?= $(REPO)/go/bin
export GOBIN := $(GOBIN)
export PATH := $(GOBIN):$(PATH)
GOCACHE ?=
SHELL := bash

export GOCACHE

all: mos fwbuild-manager fwbuild-instance

# Our main targets

mos: PKG = github.com/mongoose-os/mos/cli
mos: OUT ?= mos
mos: build-mos

fwbuild-manager: PKG = github.com/mongoose-os/mos/fwbuild/manager
fwbuild-manager: OUT ?= fwbuild-manager
fwbuild-manager: build-fwbuild-manager

fwbuild-instance: PKG = github.com/mongoose-os/mos/fwbuild/instance
fwbuild-instance: OUT ?= fwbuild-instance
fwbuild-instance: build-fwbuild-instance

$(GOBIN)/go-bindata:
	go install github.com/mongoose-os/mos/vendor/github.com/go-bindata/go-bindata/go-bindata

$(GOBIN)/go-bindata-assetfs:
	go install github.com/mongoose-os/mos/vendor/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs

vendor/modules.txt:
	go mod download
	go mod vendor

deps: vendor/modules.txt

generate: $(GOBIN)/go-bindata $(GOBIN)/go-bindata-assetfs
	go generate \
	  github.com/mongoose-os/mos/cli/... \
	  github.com/mongoose-os/mos/common/... \
	  github.com/mongoose-os/mos/fwbuild/...

version/version.go version/version.json:
	@# If we are building a Debian package, use its version.
	@# Debian package versions look like this:
	@#   1.12+92e435b~xenial0 (mos) or
	@#   201708051141+e90a9bf~xenial0 (mos-latest).
	@# The corresponding changelog entry looks like this:
	@# mos-latest (201708051141+e90a9bf~xenial0) xenial; urgency=low
	@# The part before "+" becomes version, entire string is used as build id.
	@[ -f debian/changelog ] && { \
	  head -n 1 debian/changelog | cut -d '(' -f 2 | cut -d '+' -f 1 > pkg.version; \
	  head -n 1 debian/changelog | cut -d '(' -f 2 | cut -d ')' -f 1 > pkg.build_id; \
	} || true
	tools/fw_meta.py gen_build_info \
		--version=`[ -f pkg.version ] && cat pkg.version` \
		--id=`[ -f pkg.build_id ] && cat pkg.build_id` \
		--tag_as_version=true \
		--go_output=version/version.go \
		--json_output=version/version.json
	@cat version/version.json
	@echo

version: version/version.go

get-version: version/version.json
	jq -r .build_version version/version.json

build-%: version vendor/modules.txt
	@go version
	GOOS=$(GOBUILD_GOOS) GOARCH=$(GOBUILD_GOARCH) CC=$(GOBUILD_CC) CXX=$(GOBUILD_CXX) \
	  go build -mod=vendor -tags $(GOBUILD_TAGS) -ldflags '-s -w '$(GOBUILD_LDFLAGS) -o $(OUT) $(PKG)

docker-build-%:
	docker run -i --rm \
	  -v $(CURDIR):/src \
	  --user $(shell id -u):$(shell id -g) \
	  -e HOME=/tmp \
	  -e GOBIN=/src/go/bin/$* \
	  -e GOCACHE=/src/go/.cache \
	  docker.io/mgos/ubuntu-golang:bionic \
	    make -C /src $* OUT=tools/docker/$*/$*
	  $(MAKE) -C tools/docker/$* docker-build NOBUILD=1 TAG=$(TAG)

docker-push-%:
	  $(MAKE) -C tools/docker/$* docker-push TAG=$(TAG)

docker-push-release-%:
	  $(MAKE) -C tools/docker/$* docker-tag FROM_TAG=$(TAG) TAG=release
	  $(MAKE) -C tools/docker/$* docker-push TAG=release

downloads-linux:
	docker run -i --rm \
	  -v $(CURDIR):/src \
	  --user $(shell id -u):$(shell id -g) \
	  -e GOBIN=/src/go/bin/linux \
	  docker.io/mgos/ubuntu32-golang:bionic \
	    make -C /src mos OUT=downloads/mos/linux/mos \
	    GOBUILD_TAGS='"osusergo netgo no_libudev old_libftdi"' \
	    GOBUILD_LDFLAGS='-extldflags=-static'

deps-mac:
	brew install coreutils libftdi libusb-compat pkg-config || true

downloads-mac: deps-mac mos
	mkdir -p downloads/mos/mac
	mv mos downloads/mos/mac/mos

downloads-win:
	docker run -i --rm \
	  -v $(CURDIR):/src \
	  --user $(shell id -u):$(shell id -g) \
	  -e CGO_ENABLED=1 \
	  -e GOBIN=/src/go/bin/win \
	docker.io/mgos/golang-mingw \
	  make -C /src mos OUT=downloads/mos/win/mos.exe \
	    GOBUILD_GOOS=windows \
	    GOBUILD_GOARCH=386 \
	    GOBUILD_CC=i686-w64-mingw32-gcc \
	    GOBUILD_CXX=i686-w64-mingw32-g++ \
	    GOBUILD_LDFLAGS='-extldflags=-static'

os-check:
	@[ "`uname -s`" == "Darwin" ] || \
	  { echo === Can only build downloads on a Mac, this is `uname -s`; exit 1; }

downloads: os-check clean-version clean-vendor deps downloads-linux downloads-mac downloads-win
	cp version/version.json downloads/mos/

deploy-downloads: downloads
	rsync -a --progress downloads/mos/ root@mongoose-os.com:/data/downloads/mos-$(TAG)/
ifneq "$(TAG)" "latest"
	ssh root@mongoose-os.com 'rm -f /data/downloads/mos-release && ln -vsf mos-$(TAG) /data/downloads/mos-release'
endif

uglify:
	uglifyjs --compress --mangle -- web_root/js/main.js web_root/js/util.js > /dev/null

clean: clean-tools
	rm -rf mos mos.exe mos/mos fwbuild-instance fwbuild-manager downloads/mos/{dmg,mac,linux,win} *.gz

clean-tools:
	rm -rf $(GOBIN)

clean-vendor:
	rm -rf vendor

clean-version: clean
	rm -f version/version.*

test:
	go test github.com/mongoose-os/mos/cli/...
	go test github.com/mongoose-os/mos/common/...
	go test github.com/mongoose-os/mos/fwbuild/...
