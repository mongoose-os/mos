The Mongoose OS command line tool
=================================

## Installing on Windows

Download and run [pre-built mos.exe](https://mongoose-os.com/downloads/mos-release/win/mos.exe).

## Installing on Ubuntu Linux

Use PPA:

```bash
$ sudo add-apt-repository ppa:mongoose-os/mos
$ sudo apt-get update
$ sudo apt-get install mos
```

Note: to use the very latest version instead of the released one, the last
command should be `sudo apt-get install mos-latest`

## Installing on Arch Linux

Use PKGBUILD:

```bash
$ git clone https://github.com/cesanta/mos-tool
$ cd mos-tool/mos/archlinux_pkgbuild/mos-release
$ makepkg
$ pacman -U ./mos-*.tar.xz
```

Note: to use the very latest version from the git repo, instead of the released
one, invoke `makepkg` from `mos-tool/mos/archlinux_pkgbuild/mos-latest`.

## Installing Mac OS

Use homebrew:

```bash
$ brew tap cesanta/mos
$ brew install mos
```

## Building manually

Minimal required Go version is 1.8.

Go and other required tools can be installed on Ubuntu 16.10 as follows:

```bash
sudo apt install golang-go build-essential python python-git libftdi-dev
```

Make sure you have `GOPATH` set, and `PATH` should contain `$GOPATH/bin`.
It can be done by adding this to your `~/.bashrc`:

```bash
export GOPATH="$HOME/go"
export PATH="$PATH:$GOPATH/bin"
```

Install govendor:

```bash
go get github.com/kardianos/govendor
```

Now clone the `mos-tool` repository into the proper location and `cd` to it

```bash
git clone https://github.com/cesanta/mos-tool $GOPATH/src/cesanta.com
cd $GOPATH/src/cesanta.com
```

Fetch all vendored packages and save them under the `vendor` dir:

```
$ govendor sync -v
```

Now, `mos` tool can be built:

```
make -C mos install
```

It will produce the binary `$GOPATH/bin/mos`.

## Changelog

See [release notes for this repo](https://github.com/cesanta/mos-tool/releases).

Up to version 1.25, mos tool was located under the
[mongoose-os](https://github.com/cesanta/mongoose-os) repo, so its changelog
can be found in [mongoose-os release notes](https://github.com/cesanta/mongoose-os/releases).
