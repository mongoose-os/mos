// +build !noflash

package esp32

import (
	"bytes"
	"encoding/binary"
	"strings"

	"cesanta.com/common/go/fwbundle"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
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
