#
# Copyright (c) 2015 Cesanta Software Limited
# All rights reserved
#

STUB ?= stub.json
COMMON_STUB_DIR = ../../esp/stub
SDK ?= docker.io/mgos/esp8266-build:3.0.6_1391d154-r1
LIBS ?=

SRCS = $(LIBS) $(COMMON_STUB_DIR)/stub_flasher.c $(COMMON_STUB_DIR)/slip.c

.PHONY: all

all: $(STUB)

$(STUB): $(SRCS) $(COMMON_STUB_DIR)/wrap_stub.py
	@echo "  $(SRCS) -> $@"
	docker run --rm -i -v $(CURDIR)/../..:/src $(SDK) /bin/bash -c \
    "cd /src/esp8266/stub && \
    xtensa-lx106-elf-gcc -std=c99 -Wall -Werror -Os \
         -DESP8266 -DMINIZ_NO_TIME -DMINIZ_NO_STDIO \
         -mtext-section-literals -mlongcalls -nostdlib -fno-builtin \
         -I/src/esp8266/stub \
         -I/src/esp/stub \
         -I/opt/Espressif/ESP8266_NONOS_SDK/include \
         -I/opt/Espressif/ESP8266_NONOS_SDK/driver_lib/include/driver \
         -ffunction-sections -Wl,-static -Wl,--gc-sections \
         -Tstub.ld -o /tmp/stub.elf $(SRCS) -lgcc && \
    $(COMMON_STUB_DIR)/wrap_stub.py --output=$@ /tmp/stub.elf"
