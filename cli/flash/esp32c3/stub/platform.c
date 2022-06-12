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
  // Switch to 40 MHz
  REG_SET_FIELD(SYSTEM_SYSCLK_CONF_REG, SYSTEM_PRE_DIV_CNT, 0);
  ets_update_cpu_frequency(CPU_FREQ_MHZ);

  // Enable perf insn counter with overflow.
  __asm volatile("csrwi 0x7e0, 1");  // MPCER
  __asm volatile("csrwi 0x7e1, 1");  // MPCMR

  esp_rom_spiflash_attach(ets_efuse_get_spiconfig(), 0 /* legacy */);
}
