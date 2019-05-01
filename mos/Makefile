.PHONY: all all-downloads build clean clean-version deploy deploy-fwbuild deploy-mos-binary deps downloads generate install linux mac win

REPO := $(realpath ..)
export GOPATH := $(REPO)/go
export GOCACHE := $(REPO)/go/.cache
GOBINPATH := $(GOPATH)/bin
export PATH := $(GOBINPATH):$(PATH)
SHELL := bash

PKG := github.com/mongoose-os/mos/mos
PKGDIR := $(GOPATH)/src/$(PKG)

TAG ?= latest

OUTDIR ?= $(CURDIR)

all: build

deps:
	go get github.com/kardianos/govendor
	go install github.com/kardianos/govendor
	cd $(GOPATH)/src/$(PKG) && $(GOBINPATH)/govendor sync -v

generate: version/version.go
	@[ -f $(GOBINPATH)/go-bindata ] || \
	  go install github.com/mongoose-os/mos/vendor/github.com/jteeuwen/go-bindata/go-bindata
	@[ -f $(GOBINPATH)/go-bindata-assetfs ] || \
	  go install github.com/mongoose-os/mos/vendor/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs
	@cd $(PKGDIR) && go generate ./...

version/version.go version/version.json:
	@# If we are building a Debian package, use its version.
	@# Debian package versions look like this:
	@#   1.12+92e435b~xenial0 (mos) or
	@#   201708051141+e90a9bf~xenial0 (mos-latest).
	@# The corresponding changelog entry looks like this:
	@# mos-latest (201708051141+e90a9bf~xenial0) xenial; urgency=low
	@# The part before "+" becomes version, entire string is used as build id.
	@[ -f ../debian/changelog ] && { \
	  head -n 1 ../debian/changelog | cut -d '(' -f 2 | cut -d '+' -f 1 > pkg.version; \
	  head -n 1 ../debian/changelog | cut -d '(' -f 2 | cut -d ')' -f 1 > pkg.build_id; \
	} || true
	../tools/fw_meta.py gen_build_info \
		--version=`[ -f pkg.version ] && cat pkg.version` \
		--id=`[ -f pkg.build_id ] && cat pkg.build_id` \
		--tag_as_version=true \
		--go_output=version/version.go \
		--json_output=version/version.json
	@cat version/version.json
	@echo

build: generate
	go build -v $(PKG)

install: generate
	go install -v $(PKG)

linux: generate
	docker run -i --rm \
	  -v $(GOPATH)/src/github.com/mongoose-os/mos:/go/src/github.com/mongoose-os/mos \
	  -v $(OUTDIR):/out \
	  --user $(shell id -u):$(shell id -g) \
	  docker.io/mgos/gobuild32 \
	    -tags 'netgo' \
	    -o /out/downloads/mos/linux/mos -tags no_libudev --ldflags '-extldflags "-static"' \
	    $(PKG)

mac: generate
	brew install coreutils libftdi libusb-compat pkg-config
	go build -o $(OUTDIR)/downloads/mos/mac/mos $(PKG)


win: generate
	docker run -i --rm \
	  -v $(GOPATH)/src/github.com/mongoose-os/mos:/go/src/github.com/mongoose-os/mos \
	  -v $(OUTDIR):/out \
	  --user $(shell id -u):$(shell id -g) \
	docker.io/mgos/gobuild-mingw \
	  bash -c 'GOOS=windows GOARCH=386 CGO_ENABLED=1 CXX=i686-w64-mingw32-g++ CC=i686-w64-mingw32-gcc \
	           go build -o /out/downloads/mos/win/mos.exe --ldflags "-extldflags -static" $(PKG)'

os-check:
	#@[ "`uname -s`" == "Darwin" ] || \
	#  { echo === Can only build downloads on a Mac, this is `uname -s`; exit 1; }

downloads: os-check deps clean-version generate linux mac win
	cp version/version.json $(OUTDIR)/downloads/mos/

deploy: deploy-fwbuild deploy-mos-binary

deploy-fwbuild:
	cd ../tools/ansible && \
    ansible-playbook fwbuild.yml -e build=yes -e mos_version_tag='$(TAG)' -t cloud-mos,fwbuild-instance

deploy-mos-binary: downloads
	cd ../tools/ansible && \
    ansible-playbook website.yml -e build=yes -e mos_version_tag='$(TAG)' -t mos

uglify:
	uglifyjs --compress --mangle -- web_root/js/main.js web_root/js/util.js > /dev/null

clean:
	rm -rf mos_* mos.exe mos gobuild-cache downloads/mos/{dmg,mac,linux,win}

clean-version: clean
	rm -f version/version.*
