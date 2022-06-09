/*
 * Copyright (c) 2022 Cesanta Software Limited
 * All rights reserved
 */

/* See Table 3-­2. Internal Memory Address Mapping in technical manual
 * Refer to esp32s3 bootloader.ld for memory reserved for bootloader stages
 * Segments are placed before bootloader segments
 */
MEMORY {
  iram     : org = 0x40386000, len = 0x10000
  dram     : org = 0x3FCAE000, len = 0x20000
}

ENTRY(stub_main)

SECTIONS {
  .params 0x40386000 : {
    _params_start = ABSOLUTE(.);
    *(.params)
    _params_end = ABSOLUTE(.);
  } > iram

  .text : ALIGN(4) {
    _code_start = ABSOLUTE(.);
    *(.literal)
    *(.text .text.*)
  } > iram

  .bss : ALIGN(4) {
    _bss_start = ABSOLUTE(.);
    *(.bss)
    _bss_end = ABSOLUTE(.);
  } > dram

  .data : ALIGN(4) {
    _data_start = ABSOLUTE(.);
    *(.data)
    *(.rodata .rodata.*)
  } > dram
}

INCLUDE "components/esp_rom/esp32s3/ld/esp32s3.rom.ld"
INCLUDE "components/esp_rom/esp32s3/ld/esp32s3.rom.api.ld"
INCLUDE "components/esp_rom/esp32s3/ld/esp32s3.rom.newlib.ld"
