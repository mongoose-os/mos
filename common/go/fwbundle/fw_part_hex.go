package fwbundle

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"

	"github.com/cesanta/errors"
)

type HexBundle struct {
	Parts []*HexPart
	Start uint32
}

type HexPart struct {
	Addr uint32
	Data []byte
}

func ParseHexBundle(hexData []byte) (*HexBundle, error) {
	hb := &HexBundle{}
	eof := false
	scanner := bufio.NewScanner(bytes.NewBuffer(hexData))
	lineNo := 0
	var curData []byte
	var partBase, curBase, curAddr uint32
	setPartBase := false
	for scanner.Scan() {
		lineNo++
		l := scanner.Text()
		if len(l) == 0 {
			continue
		}
		if l[0] != ':' {
			return nil, errors.Errorf("line %d: invalid start of the line", lineNo)
		}
		if len(l) < 11 || len(l)%2 != 1 {
			return nil, errors.Errorf("line %d: too short (%d)", lineNo, len(l))
		}
		ld, err := hex.DecodeString(l[1:])
		if err != nil {
			return nil, errors.Errorf("line %d: error decoding record body", lineNo)
		}
		buf := bytes.NewBuffer(ld)
		var recLen uint8
		binary.Read(buf, binary.BigEndian, &recLen)
		if len(ld) != 4+int(recLen)+1 {
			return nil, errors.Errorf("line %d: invalid length", lineNo, len(ld))
		}
		checksum := uint8(ld[len(ld)-1])
		cs := uint8(0)
		for _, b := range ld[:len(ld)-1] {
			cs += uint8(b)
		}
		cs = (cs ^ 0xff) + 1
		if cs != checksum {
			return nil, errors.Errorf("line %d: invalid checksum (want %02x, got %02x)", lineNo, checksum, cs)
		}
		var recOffset uint16
		binary.Read(buf, binary.BigEndian, &recOffset)
		var recType uint8
		binary.Read(buf, binary.BigEndian, &recType)
		switch recType {
		case 0:
			data := make([]byte, recLen)
			buf.Read(data)
			addr := curBase + uint32(recOffset)
			if !setPartBase {
				partBase = curBase
				setPartBase = true
			}
			if curData != nil && addr != curAddr {
				// There is a discontinuity in data, flush the part.
				hb.Parts = append(hb.Parts, &HexPart{
					Addr: partBase,
					Data: curData,
				})
				curBase = addr
				curData = nil
				partBase = addr
			}
			curData = append(curData, data...)
			curAddr = curBase + uint32(recOffset) + uint32(len(data))
		case 1:
			if curData != nil {
				hb.Parts = append(hb.Parts, &HexPart{
					Addr: partBase,
					Data: curData,
				})
			}
			eof = true
		case 2:
			if recLen != 2 {
				return nil, errors.Errorf("line %d: invalid extended segment address", lineNo)
			}
			var addr uint16
			binary.Read(buf, binary.BigEndian, &addr)
			curBase = uint32(addr) << 4
		case 4:
			if recLen != 2 {
				return nil, errors.Errorf("line %d: invalid extended linear address", lineNo)
			}
			var addr uint16
			binary.Read(buf, binary.BigEndian, &addr)
			curBase = uint32(addr) << 16
		case 5:
			if recLen != 4 {
				return nil, errors.Errorf("line %d: invalid start linear address", lineNo)
			}
			binary.Read(buf, binary.BigEndian, &hb.Start)
		default:
			return nil, errors.Errorf("line %d: unsupported record type (%d)", lineNo, recType)
		}
		if eof {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Annotatef(err, "line %d", lineNo)
	}
	if !eof {
		return nil, errors.Errorf("unexpected end of data")
	}
	return hb, nil
}

func PartsFromHex(hexData []byte, baseName string) ([]*FirmwarePart, error) {
	hb, err := ParseHexBundle(hexData)
	if err != nil {
		return nil, errors.Annotatef(err, "error parsing hex data")
	}
	var pp []*FirmwarePart
	for i, hp := range hb.Parts {
		name := baseName
		if i > 0 {
			name = fmt.Sprintf("%s_%d", baseName, i)
		}
		p := &FirmwarePart{
			Name: name,
			Src:  fmt.Sprintf("%s.bin", name),
			Addr: hp.Addr,
			Size: uint32(len(hp.Data)),
		}
		p.SetData(hp.Data)
		pp = append(pp, p)
	}
	return pp, nil
}

func PartsFromHexFile(fname string, baseName string) ([]*FirmwarePart, error) {
	hexData, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return PartsFromHex(hexData, baseName)
}
