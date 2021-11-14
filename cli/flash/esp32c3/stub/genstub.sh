#!/bin/bash

scriptdir=$(realpath $(dirname $0))

set -e -x

make -C "$scriptdir" \
  LIBS="spi_flash_rom_patch.c uart.c" \
  STUB="stub.json"
