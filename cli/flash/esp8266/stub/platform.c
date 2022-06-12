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

void stub_platform_init(void) {
  SelectSpiFunction();
  spi_flash_attach();
  // Switch CPU to 160 MHz
  SET_PERI_REG_MASK(0x3ff00014, 1);
  // Increase SPI flash frequency
  WRITE_PERI_REG(SPI_CLOCK_REG(0), 0x00001001);
}
