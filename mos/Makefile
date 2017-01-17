.PHONY: generate

all: generate

generate version.go version.json:
	@go install cesanta.com/vendor/github.com/jteeuwen/go-bindata/go-bindata
	@go install cesanta.com/vendor/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs
	@go generate
	@cat version.json
	@echo

install:
	go install

downloads: generate
	# mkdir -p downloads/mos/win downloads/mos/mac downloads/mos/linux
	GOOS=darwin go build -o downloads/mos/mac/mos
	GOOS=windows GOARCH=386 go build -o downloads/mos/win/mos.exe
	docker run -i --rm \
    -v $${GOPATH%%:*}/src:/go/src \
    -v $$(pwd):/out \
    -v $$(pwd)/gobuild-cache:/go/pkg \
    docker.cesanta.com/gobuild32 \
    -o /out/downloads/mos/linux/mos --ldflags '-extldflags "-static"' \
    cesanta.com/mos
	cp version.json downloads/mos/

deploy:
	cd ../prod && ansible-playbook website.yml -t mos

clean:
	rm -rf mos_* mos.exe mos gobuild-cache downloads version*
