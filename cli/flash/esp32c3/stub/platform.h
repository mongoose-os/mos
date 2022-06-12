#pragma once

#include "soc/spi_mem_reg.h"

#include "esp32c3/rom/efuse.h"
#include "esp32c3/rom/ets_sys.h"
#include "esp32c3/rom/md5_hash.h"
#include "esp32c3/rom/miniz.h"
#include "esp32c3/rom/rtc.h"
#include "esp32c3/rom/spi_flash.h"
#include "esp32c3/rom/uart.h"

#include "soc/gpio_reg.h"
#include "soc/spi_mem_reg.h"
#include "soc/system_reg.h"
#include "soc/uart_reg.h"

#define CPU_FREQ_MHZ 40
#define LED_GPIO 3

void stub_platform_init(void);

static inline void stub_spi_flash_wait_idle(void) {
  esp_rom_spiflash_wait_idle(&g_rom_flashchip);
}

static inline uint32_t stub_get_ccount(void) {
  uint32_t r;
  __asm volatile("csrr %0, 0x7e2" : "=r"(r));  // MPCCR
  return r;
}
