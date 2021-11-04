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
 * Windows 10:
   * Install [MSYS2](https://www.msys2.org/)
   * `pacman -S --needed base-devel mingw-w64-x86_64-toolchain git mingw-w64-x86_64-go python mingw-w64-x86_64-libftdi mingw-w64-x86_64-libusb pkgconf`
   * Use a MSYS MinGW 64-bit shell

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

## Setup a private build server ( Ubuntu )

Make sure you have Docker installed on machine you want the server to run:

```
$ sudo apt-get install docker docker-compose
```

Clone the mos repostory:

```
$ git clone https://github.com/mongoose-os/mos
$ cd mos/fwbuild
```

Run build server:

```
$ sudo docker-compose up
```

If you are running any firewall on your network or using EC2 instance - remember to add an exception to allow incoming connections to the server on port 8000

Run the build from your local machine:

```
$ mos build --platfrom <YOUR PLATFORM HERE> --server http://IP:8000
```
If you receive an error that files is too big - remove you local deps folder and run the build again. 
By default your machien will try to send to build server all the files with libraries together with the source code what makes the file pretty big. 
If you remove the local deps folder it will only send your source files and pull all necessery libraries remotely.

Once the build is complete your newly built firmware will be sent back to your ./build/fw directory and is ready to be flash to your local device.
