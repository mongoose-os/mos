/*
 * Copyright (c) 2016 Cesanta Software Limited
 * All rights reserved
 */

/* See Table 3-­2. Internal Memory Address Mapping in technical manual
 * Refer to esp32c3 bootloader.ld for memory reserved for bootloader stages
 * Segments are placed before bootloader segments
 */
MEMORY {
  iram     : org = 0x403BE000, len = 0x10000
  dram_bss : org = 0x3FCB6000, len = 0x10000
  dram     : org = 0x3FCC6000, len = 0x10000
}

ENTRY(stub_main)

SECTIONS {
  .params 0x403BE000 : {
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

INCLUDE "components/esp_rom/esp32c3/ld/esp32c3.rom.ld"

PROVIDE ( memcpy = 0x40000358 );
PROVIDE ( memset = 0x40000354 );
PROVIDE ( MD5Init = 0x40000614 );
PROVIDE ( MD5Update = 0x40000618 );
PROVIDE ( MD5Final = 0x4000061c );
PROVIDE ( ets_isr_mask = 0x40000608 );
PROVIDE ( ets_isr_unmask = 0x4000060c );
PROVIDE ( esp_rom_spiflash_erase_area = 0x400001e4 );
