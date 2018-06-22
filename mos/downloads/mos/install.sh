#!/bin/bash

OS=`uname`
PROGNAME=mos
VERSION=${1:-release}  # Can also be "latest" or a version number
DESTDIR=${2:-~/.mos/bin}

checklib() {
  if ! test -f "$1" ; then
    echo "Installing `basename $1` ..."
    mkdir -p `dirname $1`
    curl -fsSL https://mongoose-os.com/downloads/mos-$VERSION/`basename $1` -o "$1"
  fi
}

test -d $DESTDIR || mkdir -p $DESTDIR

FULLPATH=$DESTDIR/$PROGNAME

if test "$OS" = Linux ; then
  MOS_URL=https://mongoose-os.com/downloads/mos-$VERSION/linux/mos
elif test "$OS" = Darwin ; then
  MOS_URL=https://mongoose-os.com/downloads/mos-$VERSION/mac/mos
  checklib /usr/local/opt/libftdi/lib/libftdi1.2.dylib
  checklib /usr/local/opt/libusb/lib/libusb-1.0.0.dylib
else
  echo "Unsupported OS [$OS]. Only Linux or MacOS are supported."
  echo "FAILURE, exiting."
  exit 1
fi

if ! test -d $DESTDIR ; then
  echo "Directory $DESTDIR is not present, creating it ..."
  mkdir -p $DESTDIR
fi

echo "Downloading $MOS_URL ..."
curl -L --progress-bar $MOS_URL -o $FULLPATH

echo "Installing into $FULLPATH ..."
chmod 755 $FULLPATH

mos --help 2>/dev/null
if test "$?" == "127" ; then
  RC_FILE=$HOME/.bashrc
  test -f $RC_FILE || RC_FILE=$HOME/.profile
  echo "Adding $DESTDIR to your PATH in $RC_FILE"
  echo "PATH=\"\$PATH:$DESTDIR\"" >> $RC_FILE
fi

echo "SUCCESS: $FULLPATH is installed."
echo "Run '$FULLPATH --help' to see all available commands."
echo "Run '$FULLPATH' without arguments to start a simplified Web UI installer."
