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
$ git clone https://github.com/mongoose-os/mos
$ cd mos/mos/archlinux_pkgbuild/mos-release/
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

You will need:
 * Git
 * Go version 1.10 or later
 * GNU Make
 * Python 3
 * libftdi + headers
 * libusb 1.0 + headers
 * Docker - optional, only for building Windows binaries on Mac or Linux.

Commands to install all the build dependencies:
 * Ubuntu Linux: `sudo apt-get install build-essential git golang-go python3 libftdi-dev libusb-1.0-0-dev pkg-config`
 * Mac OS X (via [Homebrew](https://brew.sh/)): `brew install coreutils libftdi libusb-compat pkg-config`
 * Windows 10: `TODO`

Clone the repo (note: doesn't have to be in `GOPATH`):

```
$ git clone https://github.com/mongoose-os/mos
$ cd mos
```

Fetch dependencies (only needed for the first build):

```
$ make deps
```

Build the binary:

```
$ make
```

It will produce `mos/mos` (or mos/mos.exe` on Windows.
