/*
 * Copyright (c) 2014-2022 Cesanta Software Limited
 * All rights reserved
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include "platform.h"

static uint32_t get_chip_pkg(void) {
  uint32_t a = REG_GET_FIELD(EFUSE_BLK0_RDATA3_REG, EFUSE_RD_CHIP_VER_PKG);
  uint32_t b = REG_GET_FIELD(EFUSE_BLK0_RDATA3_REG, EFUSE_RD_CHIP_VER_PKG_4BIT);
  return (b << 3) | a;
}

void stub_platform_init(void) {
  esp_rom_spiflash_attach(ets_efuse_get_spiconfig(), 0 /* legacy */);
  // Increase SPI clock frequency. Devices with external flash
  // are able to handle 40 MHz (CPU_CLK), ones with internal flash cannot.
  // For them we still increase the speed a bit by reducing the divider.
  switch (get_chip_pkg()) {
    case 2:  // ESP32D2WD
    case 4:  // ESP32-U4WDH
    case 5:  // ESP32-PICO-V3 or ESP32-PICO-D4
    case 6:  // ESP32-PICO-V3-02
      WRITE_PERI_REG(SPI_CLOCK_REG(1), 0x00002002);
      break;
    default:
      SET_PERI_REG_MASK(SPI_CLOCK_REG(1), SPI_CLK_EQU_SYSCLK);
      break;
  }
}
