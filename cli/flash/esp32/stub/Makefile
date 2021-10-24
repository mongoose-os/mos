#
# Copyright (c) 2015 Cesanta Software Limited
# All rights reserved
#

STUB ?= stub.json
COMMON_STUB_DIR = ../../esp/stub
SDK ?= docker.io/mgos/esp32-build:4.2-r4
LIBS ?=

SRCS = $(LIBS) $(COMMON_STUB_DIR)/stub_flasher.c $(COMMON_STUB_DIR)/slip.c

.PHONY: all

all: $(STUB)

$(STUB): $(SRCS) $(COMMON_STUB_DIR)/wrap_stub.py
	@echo "  $(SRCS) -> $@"
	docker run --rm -i -v $(CURDIR)/../..:/src $(SDK) /bin/bash -c \
    "cd /src/esp32/stub && \
    xtensa-esp32-elf-gcc -std=c99 -Wall -Werror -Os -DESP32 \
         -mtext-section-literals -mlongcalls -nostdlib -fno-builtin \
         -DCONFIG_IDF_TARGET_ESP32 \
         -I/src/esp32/stub \
         -I/opt/Espressif/esp-idf/components/esp32/include \
         -I/opt/Espressif/esp-idf/components/esp_common/include \
         -I/opt/Espressif/esp-idf/components/esp_rom/include \
         -I/opt/Espressif/esp-idf/components/soc/soc/include \
         -I/opt/Espressif/esp-idf/components/soc/soc/esp32/include \
         -I/opt/Espressif/esp-idf/components/xtensa/include \
         -L/opt/Espressif/esp-idf \
         -ffunction-sections -Wl,-static -Wl,--gc-sections \
         -Tstub.ld -o /tmp/stub.elf $(SRCS) && \
    $(COMMON_STUB_DIR)/wrap_stub.py --output=$@ /tmp/stub.elf"
