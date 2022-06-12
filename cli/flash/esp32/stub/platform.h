#pragma once

#include "soc/spi_reg.h"

#include "esp32/rom/efuse.h"
#include "esp32/rom/ets_sys.h"
#include "esp32/rom/md5_hash.h"
#include "esp32/rom/miniz.h"
#include "esp32/rom/rtc.h"
#include "esp32/rom/spi_flash.h"
#include "esp32/rom/uart.h"

#include "soc/efuse_reg.h"
#include "soc/spi_reg.h"
#include "soc/uart_reg.h"

#define LED_GPIO 5
#define CPU_FREQ_MHZ 160

extern esp_rom_spiflash_chip_t g_rom_flashchip;

void stub_platform_init(void);

static inline void stub_spi_flash_wait_idle(void) {
  esp_rom_spiflash_wait_idle(&g_rom_flashchip);
}

static inline uint32_t stub_get_ccount(void) {
  uint32_t r;
  __asm volatile("rsr.ccount %0" : "=a"(r));
  return r;
}

#define SPI_MEM_CMD_REG(i) SPI_CMD_REG(i)
#define SPI_MEM_FLASH_BE SPI_FLASH_BE
#define SPI_MEM_FLASH_SE SPI_FLASH_SE
#define SPI_MEM_FLASH_RDID SPI_FLASH_RDID
#define SPI_MEM_FLASH_WREN SPI_FLASH_WREN
#define SPI_MEM_USR_DUMMY SPI_USR_DUMMY
#define SPI_MEM_USR_ADDR_BITLEN SPI_USR_ADDR_BITLEN
#define SPI_MEM_USR_ADDR_BITLEN_S SPI_USR_ADDR_BITLEN_S
#define SPI_MEM_USR_ADDR_BITLEN_V SPI_USR_ADDR_BITLEN_V

uint32_t stub_read_flash_id(void);
