/*
 * Copyright (c) 2014-2018 Cesanta Software Limited
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

#pragma once

#include <inttypes.h>

#include "c_types.h"
#include "eagle_soc.h"
#include "ets_sys.h"
#include "miniz.h"
#include "spi_flash.h"

int uart_rx_one_char(uint8_t *ch);
uint8_t uart_rx_one_char_block();
int uart_tx_one_char(char ch);
void uart_div_modify(uint32_t uart_no, uint32_t baud_div);

int SendMsg(uint8_t *msg, uint8_t size);
int send_packet(uint8_t *packet, uint32_t size);
// recv_packet depends on global UartDev, better to avoid it.
// uint32_t recv_packet(void *packet, uint32_t len, uint8_t no_sync);

void _putc1(char *ch);

void ets_delay_us(uint32_t us);

uint32_t SPILock();
uint32_t SPIUnlock();
uint32_t SPIRead(uint32_t addr, void *dst, uint32_t size);
uint32_t SPIWrite(uint32_t addr, const uint32_t *src, uint32_t size);
uint32_t SPIEraseChip();
uint32_t SPIEraseBlock(uint32_t block_num);
uint32_t SPIEraseSector(uint32_t sector_num);

extern SpiFlashChip *flashchip;
uint32_t Wait_SPI_Idle(SpiFlashChip *spi);
uint32_t SPI_chip_erase(SpiFlashChip *spi);
uint32_t SPI_read_status(SpiFlashChip *spi);
uint32_t SPI_write_enable(SpiFlashChip *spi);

void spi_flash_attach();

/* ESP32 API compatibility */
#define esp_rom_spiflash_unlock SPIUnlock
#define esp_rom_spiflash_erase_sector SPIEraseSector
#define esp_rom_spiflash_erase_block SPIEraseBlock
#define esp_rom_spiflash_erase_chip SPIEraseChip
#define esp_rom_spiflash_read SPIRead
#define esp_rom_spiflash_write SPIWrite
#define esp_rom_spiflash_config_param SPIParamCfg

void SelectSpiFunction();
void SPIFlashModeConfig(uint32_t a, uint32_t b);
void SPIReadModeCnfig(uint32_t a);
uint32_t SPIParamCfg(uint32_t deviceId, uint32_t chip_size, uint32_t block_size,
                     uint32_t sector_size, uint32_t page_size,
                     uint32_t status_mask);

void Cache_Read_Disable();

void ets_delay_us(uint32_t delay_micros);

void ets_isr_mask(uint32_t ints);
void ets_isr_unmask(uint32_t ints);
typedef void (*int_handler_t)(void *arg);

void ets_intr_lock();
void ets_intr_unlock();
void ets_set_user_start(void (*user_start_fn)());

uint32_t rtc_get_reset_reason();
void software_reset();
void rom_phy_reset_req();

void uart_rx_intr_handler(void *arg);

void _ResetVector();

/* Crypto functions are from wpa_supplicant. */
int md5_vector(uint32_t num_msgs, const uint8_t *msgs[],
               const uint32_t *msg_lens, uint8_t *digest);
int sha1_vector(uint32_t num_msgs, const uint8_t *msgs[],
                const uint32_t *msg_lens, uint8_t *digest);

struct MD5Context {
  uint32_t buf[4];
  uint32_t bits[2];
  uint8_t in[64];
};

void MD5Init(struct MD5Context *ctx);
void MD5Update(struct MD5Context *ctx, void *buf, uint32_t len);
void MD5Final(uint8_t digest[16], struct MD5Context *ctx);

#define CPU_FREQ_MHZ 160

void stub_platform_init(void);

static inline void stub_spi_flash_wait_idle(void) {
  Wait_SPI_Idle(flashchip);
}

static inline uint32_t stub_get_ccount(void) {
  uint32_t r;
  __asm volatile("rsr.ccount %0" : "=a"(r));
  return r;
}

#define REG_SPI_BASE(i) (0x60000200 - i * 0x100)
#define PERIPHS_SPI_FLASH_ADDR (REG_SPI_BASE(0) + 0x4)
#define PERIPHS_SPI_FLASH_CMD (REG_SPI_BASE(0) + 0x0)
#define PERIPHS_SPI_FLASH_C0 (REG_SPI_BASE(0) + 0x40)
#define SPI_MEM_FLASH_RDSR (BIT(27))
#define SPI_MEM_FLASH_WREN (BIT(30))
#define SPI_MEM_FLASH_RDID (BIT(28))
#define SPI_MEM_FLASH_SE (BIT(24))
#define SPI_MEM_FLASH_BE (BIT(23))

#define GPIO_OUT_REG (PERIPHS_GPIO_BASEADDR + GPIO_OUT_ADDRESS)
#define GPIO_ENABLE_W1TS_REG (PERIPHS_GPIO_BASEADDR + GPIO_ENABLE_W1TS_ADDRESS)
#define GPIO_OUT_W1TC_REG (PERIPHS_GPIO_BASEADDR + GPIO_OUT_W1TC_ADDRESS)
#define GPIO_OUT_W1TS_REG (PERIPHS_GPIO_BASEADDR + GPIO_OUT_W1TS_ADDRESS)

#define UART_RXFIFO_CNT_S 0
#define UART_RXFIFO_CNT_V 0xff

#define REG_GET_FIELD(_r, _f) ((READ_PERI_REG(_r) >> (_f##_S)) & (_f##_V))

#define LED_GPIO 5
