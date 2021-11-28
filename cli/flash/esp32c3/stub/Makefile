#
# Copyright (c) 2015 Cesanta Software Limited
# All rights reserved
#

STUB ?= stub.json
COMMON_STUB_DIR = ../../esp/stub
SDK ?= docker.io/mgos/esp32c3-build:4.3-dev
LIBS ?=

SRCS = $(LIBS) $(COMMON_STUB_DIR)/stub_flasher.c $(COMMON_STUB_DIR)/slip.c

.PHONY: all

all: $(STUB)

$(STUB): $(SRCS) $(COMMON_STUB_DIR)/wrap_stub.py
	@echo "  $(SRCS) -> $@"
	docker run --rm -i -v '$(CURDIR)/../..:/src' $(SDK) /bin/bash -c \
    "cd /src/esp32c3/stub && \
    riscv32-esp-elf-gcc -std=c99 -Wall -Werror -Os -DESP32C3 \
         -nostdlib -fno-builtin \
         -DCONFIG_IDF_TARGET_ESP32C3 -DCONFIG_IDF_TARGET_ARCH_RISCV \
         -I/src/esp32c3/stub \
         -I/opt/Espressif/esp-idf/components/esp32c3/include \
         -I/opt/Espressif/esp-idf/components/esp_common/include \
         -I/opt/Espressif/esp-idf/components/esp_rom/include \
         -I/opt/Espressif/esp-idf/components/soc/include \
         -I/opt/Espressif/esp-idf/components/soc/esp32c3/include \
         -I/opt/Espressif/esp-idf/components/spi_flash/private_include \
         -I/opt/Espressif/esp-idf/components/riscv/include \
         -L/opt/Espressif/esp-idf \
         -ffunction-sections -Wl,-static -Wl,--gc-sections \
         -Tstub.ld -o /tmp/stub.elf $(SRCS) && \
    $(COMMON_STUB_DIR)/wrap_stub.py --output=$@ /tmp/stub.elf"
