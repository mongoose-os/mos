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
$ cd mos/tools/archlinux_pkgbuild/mos-release
$ makepkg -fsri
```

Note: to use the very latest version from the git repo, instead of the released
one, invoke `makepkg` from `mos/tools/archlinux_pkgbuild/mos-latest`.

## Installing on Mac OS

Using [Homebrew][https://brew.sh/]:

```bash
$ brew tap cesanta/mos
$ brew install mos
```

To use latest:

```
$ brew install mos-latest
```

## Building manually

You will need:
 * Git
 * Go version 1.13 or later
 * GNU Make
 * Python 3
 * libftdi + headers
 * libusb 1.0 + headers
 * GCC
 * pkg-config
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

Build the binary:

```
$ make
```

It will produce `mos` (or `mos.exe` on Windows).
