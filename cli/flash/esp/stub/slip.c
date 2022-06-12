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

#include "platform.h"

void SLIP_send(uint8_t *pkt, uint32_t size) {
  send_packet(pkt, size);
}

uint32_t SLIP_recv(void *pkt, uint32_t max_len) {
  uint8_t c;
  uint32_t len = 0;
  uint8_t *p = (uint8_t *) pkt;
  do {
    c = uart_rx_one_char_block();
  } while (c != '\xc0');
  while (len < max_len) {
    c = uart_rx_one_char_block();
    if (c == '\xc0') return len;
    if (c == '\xdb') {
      c = uart_rx_one_char_block();
      if (c == '\xdc') {
        c = '\xc0';
      } else if (c == '\xdd') {
        c = '\xdb';
      } else {
        len = 0;
        break; /* Bad esc sequence. */
      }
    }
    *p++ = c;
    len++;
  }
  do {
    c = uart_rx_one_char_block();
  } while (c != '\xc0');
  return len;
}
