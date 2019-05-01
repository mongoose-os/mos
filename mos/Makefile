.PHONY: all all-downloads build clean clean-version deploy deploy-fwbuild deploy-mos-binary downloads generate install linux mac win

TAG ?= latest

OUTDIR ?= $(CURDIR)
TMP_GOPATH ?= ~/tmp/mos_gopath

all: build

generate: version/version.go
	@which go-bindata > /dev/null || \
	  go install cesanta.com/vendor/github.com/jteeuwen/go-bindata/go-bindata
	@which go-bindata-assetfs > /dev/null || \
	  go install cesanta.com/vendor/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs
	@go generate ./...

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
	../common/tools/fw_meta.py gen_build_info \
		--version=`[ -f pkg.version ] && cat pkg.version` \
		--id=`[ -f pkg.build_id ] && cat pkg.build_id` \
		--tag_as_version=true \
		--go_output=version/version.go \
		--json_output=version/version.json
	@cat version/version.json
	@echo

build: generate
	go build -v -i

install: generate
	go install

linux: generate
	docker run -i --rm \
	  -v $${GOPATH%%:*}/src:/go/src \
	  -v $(OUTDIR):/out \
	  docker.io/mgos/gobuild32 \
	    -tags 'netgo' \
	    -o /out/downloads/mos/linux/mos -tags no_libudev --ldflags '-extldflags "-static"' \
	    cesanta.com/mos

mac: generate
	brew install coreutils libftdi libusb-compat pkg-config
	go build -o $(OUTDIR)/downloads/mos/mac/mos


win: generate
	docker run -i --rm \
	  -v $${GOPATH%%:*}:/go \
	  -v $(OUTDIR):/out \
	  --user $(shell id -u):$(shell id -g) \
	docker.io/mgos/gobuild-mingw \
	  bash -c 'GOOS=windows GOARCH=386 CGO_ENABLED=1 CXX=i686-w64-mingw32-g++ CC=i686-w64-mingw32-gcc \
	           go build -v -o /out/downloads/mos/win/mos.exe --ldflags "-extldflags -static" cesanta.com/mos'

# Set up GOPATH on the side and build there. Because Go is awesome.
downloads: clean-version version/version.go
	@test "`uname -s`" == "Darwin" || \
	  { echo Can only build downloads on a Mac, this is `uname -s`; \
	   exit 1; }
	rm -rf $(TMP_GOPATH) && mkdir -p $(TMP_GOPATH)/src
	rsync -a $(CURDIR)/../ $(TMP_GOPATH)/src/cesanta.com/
	cd $(TMP_GOPATH)/src/cesanta.com && GOPATH=$(TMP_GOPATH) govendor sync -v
	rsync -a --delete $(TMP_GOPATH)/src/cesanta.com/vendor/ $(CURDIR)/../vendor/
	cd $(TMP_GOPATH)/src/cesanta.com/mos && \
	  GOPATH=$(TMP_GOPATH) OUTDIR=$(CURDIR) $(MAKE) all-downloads

all-downloads: mac linux win
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
