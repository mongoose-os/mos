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
// +build !noflash

package esp32

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/fwbundle"
)

// esp_partition_info_t
// From https://github.com/espressif/esp-idf/blob/master/components/esp32/include/esp_flash_data_types.h#L45

const (
	ESPPartitionMagic         uint16 = 0x50aa
	espPartitionTablePartName string = "pt"
)

type ESPPartitionPos struct {
	Offset uint32
	Size   uint32
}

type ESPPartitionInfo struct {
	Magic   uint16
	Type    uint8
	Subtype uint8
	Pos     ESPPartitionPos
	Label   [16]byte
	Flags   uint32
}

func GetPartitionInfo(fw *fwbundle.FirmwareBundle, name string) (*ESPPartitionInfo, error) {
	data, err := fw.GetPartData(espPartitionTablePartName)
	if err != nil {
		return nil, errors.Errorf("no partition table in the fw bundle")
	}
	ptb := bytes.NewBuffer(data)
	for {
		var pte ESPPartitionInfo
		binary.Read(ptb, binary.LittleEndian, &pte.Magic)
		if pte.Magic != ESPPartitionMagic {
			break
		}
		binary.Read(ptb, binary.LittleEndian, &pte.Type)
		binary.Read(ptb, binary.LittleEndian, &pte.Subtype)
		binary.Read(ptb, binary.LittleEndian, &pte.Pos.Offset)
		binary.Read(ptb, binary.LittleEndian, &pte.Pos.Size)
		ptb.Read(pte.Label[:])
		binary.Read(ptb, binary.LittleEndian, &pte.Flags)
		ptn := strings.TrimRight(string(pte.Label[:]), "\x00")
		glog.V(2).Infof("pt %q - %d @ 0x%x", ptn, pte.Pos.Size, pte.Pos.Offset)
		if ptn == name {
			return &pte, nil
		}
	}
	return nil, errors.Errorf("partition %q not found", name)
}
