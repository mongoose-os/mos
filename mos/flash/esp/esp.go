//
// Copyright (c) 2014-2019 Cesanta Software Limited
// All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package esp

import "fmt"

type ChipType int

const (
	ChipESP8266 ChipType = iota
	ChipESP32
)

type FlashOpts struct {
	ControlPort            string
	DataPort               string
	ROMBaudRate            uint
	FlasherBaudRate        uint
	InvertedControlLines   bool
	FlashParams            string
	EraseChip              bool
	EnableCompression      bool
	MinimizeWrites         bool
	BootFirmware           bool
	ESP32EncryptionKeyFile string
	ESP32FlashCryptConf    uint32
	KeepFS                 bool
}

type RegReaderWriter interface {
	ReadReg(reg uint32) (uint32, error)
	WriteReg(reg, value uint32) error
	Disconnect()
}

func (ct ChipType) String() string {
	switch ct {
	case ChipESP8266:
		return "ESP8266"
	case ChipESP32:
		return "ESP32"
	default:
		return fmt.Sprintf("???(%d)", ct)
	}
}
