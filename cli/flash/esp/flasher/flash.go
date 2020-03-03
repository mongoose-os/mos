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
package flasher

import (
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"math/bits"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/fwbundle"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/flash/common"
	"github.com/mongoose-os/mos/cli/flash/esp"
	"github.com/mongoose-os/mos/cli/flash/esp32"
)

const (
	flashSectorSize   = 0x1000
	flashBlockSize    = 0x10000
	sysParamsPartType = "sys_params"
	sysParamsAreaSize = 4 * flashSectorSize
	espImageMagicByte = 0xe9
)

type image struct {
	Name         string
	Type         string
	Addr         uint32
	Data         []byte
	ESP32Encrypt bool
}

type imagesByAddr []*image

func (pp imagesByAddr) Len() int      { return len(pp) }
func (pp imagesByAddr) Swap(i, j int) { pp[i], pp[j] = pp[j], pp[i] }
func (pp imagesByAddr) Less(i, j int) bool {
	return pp[i].Addr < pp[j].Addr
}

func enDis(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func Flash(ct esp.ChipType, fw *fwbundle.FirmwareBundle, opts *esp.FlashOpts) error {

	if opts.KeepFS && opts.EraseChip {
		return errors.Errorf("--keep-fs and --esp-erase-chip are incompatible")
	}

	cfr, err := ConnectToFlasherClient(ct, opts)
	if err != nil {
		return errors.Trace(err)
	}
	defer cfr.rc.Disconnect()

	if ct == esp.ChipESP8266 {
		// Based on our knowledge of flash size, adjust type=sys_params image.
		adjustSysParamsLocation(fw, cfr.flashParams.Size())
	}

	// Sort images by address
	var images []*image
	for _, p := range fw.Parts {
		if p.Type == fwbundle.FSPartType && opts.KeepFS {
			continue
		}
		data, err := fw.GetPartData(p.Name)
		if err != nil {
			return errors.Annotatef(err, "%s: failed to get data", p.Name)
		}
		// For ESP32, resolve partition name to address
		if p.ESP32PartitionName != "" {
			pti, err := esp32.GetPartitionInfo(fw, p.ESP32PartitionName)
			if err != nil {
				return errors.Annotatef(err, "%s: failed to get respolve partition %q", p.Name, p.ESP32PartitionName)
			}
			glog.V(1).Infof("%s -> %s -> 0x%x", p.Name, p.ESP32PartitionName, pti.Pos.Offset)
			p.Addr = pti.Pos.Offset
		}
		im := &image{
			Name:         p.Name,
			Type:         p.Type,
			Addr:         p.Addr,
			Data:         data,
			ESP32Encrypt: p.ESP32Encrypt,
		}
		images = append(images, im)
	}

	return errors.Trace(writeImages(ct, cfr, images, opts, true))
}

func writeImages(ct esp.ChipType, cfr *cfResult, images []*image, opts *esp.FlashOpts, sanityCheck bool) error {
	var err error

	common.Reportf("Flash size: %d, params: %s", cfr.flashParams.Size(), cfr.flashParams)

	encryptionEnabled := false
	secureBootEnabled := false
	var esp32EncryptionKey []byte
	var fusesByName map[string]*esp32.Fuse
	kcs := esp32.KeyEncodingSchemeNone
	if ct == esp.ChipESP32 {
		_, _, fusesByName, err = esp32.ReadFuses(cfr.fc)
		if err != nil {
			return errors.Annotatef(err, "failed to read eFuses")
		}

		if fcnt, err := fusesByName[esp32.FlashCryptCntFuseName].Value(true /* withDiffs */); err == nil {
			encryptionEnabled = (bits.OnesCount64(fcnt.Uint64())%2 != 0)
			kcs = esp32.GetKeyEncodingScheme(fusesByName)
			common.Reportf("Flash encryption: %s, scheme: %s", enDis(encryptionEnabled), kcs)
		}

		if abs0, err := fusesByName[esp32.AbstractDone0FuseName].Value(true /* withDiffs */); err == nil {
			secureBootEnabled = (abs0.Int64() != 0)
			common.Reportf("Secure boot: %s", enDis(secureBootEnabled))
		}
	}

	for _, im := range images {
		if im.Addr == 0 || im.Addr == 0x1000 && len(im.Data) >= 4 && im.Data[0] == 0xe9 {
			im.Data[2], im.Data[3] = cfr.flashParams.Bytes()
		}
		if ct == esp.ChipESP32 && im.ESP32Encrypt && encryptionEnabled {
			if esp32EncryptionKey == nil {
				if opts.ESP32EncryptionKeyFile != "" {
					mac := strings.ToUpper(strings.Replace(fusesByName[esp32.MACAddressFuseName].MACAddressString(), ":", "", -1))
					ekf := moscommon.ExpandPlaceholders(opts.ESP32EncryptionKeyFile, "?", mac)
					common.Reportf("Flash encryption key: %s", ekf)
					esp32EncryptionKey, err = ioutil.ReadFile(ekf)
					if err != nil {
						return errors.Annotatef(err, "failed to read encryption key")
					}
				} else {
					return errors.Errorf("flash encryption is enabled but encryption key is not provided")
				}
			}
			encrKey := esp32EncryptionKey[:]
			switch kcs {
			case esp32.KeyEncodingSchemeNone:
				if len(esp32EncryptionKey) != 32 {
					return errors.Errorf("encryption key must be 32 bytes, got %d", len(esp32EncryptionKey))
				}
			case esp32.KeyEncodingScheme34:
				if len(esp32EncryptionKey) != 24 {
					return errors.Errorf("encryption key must be 24 bytes, got %d", len(esp32EncryptionKey))
				}
				// Extend the key, per 3/4 encoding scheme.
				encrKey = append(encrKey, encrKey[8:16]...)
			}
			encData, err := esp32.ESP32EncryptImageData(
				im.Data, encrKey, im.Addr, opts.ESP32FlashCryptConf)
			if err != nil {
				return errors.Annotatef(err, "%s: failed to encrypt", im.Name)
			}
			im.Data = encData
		}
	}
	sort.Sort(imagesByAddr(images))

	if sanityCheck {
		err = sanityCheckImages(ct, images, cfr.flashParams.Size(), flashSectorSize)
		if err != nil {
			return errors.Trace(err)
		}
	}

	imagesToWrite := images
	if opts.EraseChip {
		common.Reportf("Erasing chip...")
		if err = cfr.fc.EraseChip(); err != nil {
			return errors.Annotatef(err, "failed to erase chip")
		}
	} else if opts.MinimizeWrites {
		common.Reportf("Deduping...")
		imagesToWrite, err = dedupImages(cfr.fc, images)
		if err != nil {
			return errors.Annotatef(err, "failed to dedup images")
		}
	}

	if len(imagesToWrite) > 0 {
		common.Reportf("Writing...")
		start := time.Now()
		totalBytesWritten := 0
		for _, im := range imagesToWrite {
			data := im.Data
			numAttempts := 3
			imageBytesWritten := 0
			addr := im.Addr
			if len(data)%flashSectorSize != 0 {
				newData := make([]byte, len(data))
				copy(newData, data)
				paddingLen := flashSectorSize - len(data)%flashSectorSize
				for i := 0; i < paddingLen; i++ {
					newData = append(newData, 0xff)
				}
				data = newData
			}
			for i := 1; imageBytesWritten < len(im.Data); i++ {
				common.Reportf("  %7d @ 0x%x", len(data), addr)
				bytesWritten, err := cfr.fc.Write(addr, data, true /* erase */, opts.EnableCompression)
				if err != nil {
					if bytesWritten >= flashSectorSize {
						// We made progress, restart the retry counter.
						i = 1
					}
					err = errors.Annotatef(err, "write error (attempt %d/%d)", i, numAttempts)
					if i >= numAttempts {
						return errors.Annotatef(err, "%s: failed to write", im.Name)
					}
					glog.Warningf("%s", err)
					if err := cfr.fc.Sync(); err != nil {
						return errors.Annotatef(err, "lost connection with the flasher")
					}
					// Round down to sector boundary
					bytesWritten = bytesWritten - (bytesWritten % flashSectorSize)
					data = data[bytesWritten:]
				}
				imageBytesWritten += bytesWritten
				addr += uint32(bytesWritten)
			}
			totalBytesWritten += len(im.Data)
		}
		seconds := time.Since(start).Seconds()
		bytesPerSecond := float64(totalBytesWritten) / seconds
		common.Reportf("Wrote %d bytes in %.2f seconds (%.2f KBit/sec)", totalBytesWritten, seconds, bytesPerSecond*8/1024)
	}

	common.Reportf("Verifying...")
	for _, im := range images {
		common.Reportf("  %7d @ 0x%x", len(im.Data), im.Addr)
		digest, err := cfr.fc.Digest(im.Addr, uint32(len(im.Data)), 0 /* blockSize */)
		if err != nil {
			return errors.Annotatef(err, "%s: failed to compute digest %d @ 0x%x", im.Name, len(im.Data), im.Addr)
		}
		if len(digest) != 1 || len(digest[0]) != 16 {
			return errors.Errorf("unexpected digest packetresult %+v", digest)
		}
		digestHex := strings.ToLower(hex.EncodeToString(digest[0]))
		expectedDigest := md5.Sum(im.Data)
		expectedDigestHex := strings.ToLower(hex.EncodeToString(expectedDigest[:]))
		if digestHex != expectedDigestHex {
			return errors.Errorf("%d @ 0x%x: digest mismatch: expected %s, got %s", len(im.Data), im.Addr, expectedDigestHex, digestHex)
		}
	}
	if opts.BootFirmware {
		common.Reportf("Booting firmware...")
		if err = cfr.fc.BootFirmware(); err != nil {
			return errors.Annotatef(err, "failed to reboot into firmware")
		}
	}
	return nil
}

func adjustSysParamsLocation(fw *fwbundle.FirmwareBundle, flashSize int) {
	sysParamsAddr := uint32(flashSize - sysParamsAreaSize)
	for _, p := range fw.Parts {
		if p.Type != sysParamsPartType {
			continue
		}
		if p.Addr != sysParamsAddr {
			glog.Infof("Sys params image moved from 0x%x to 0x%x", p.Addr, sysParamsAddr)
			p.Addr = sysParamsAddr
		}
	}
}

func sanityCheckImages(ct esp.ChipType, images []*image, flashSize, flashSectorSize int) error {
	// Note: we require that images are sorted by address.
	sort.Sort(imagesByAddr(images))
	for i, im := range images {
		imageBegin := int(im.Addr)
		imageEnd := imageBegin + len(im.Data)
		if imageBegin >= flashSize || imageEnd > flashSize {
			return errors.Errorf(
				"Image %d @ 0x%x will not fit in flash (size %d)", len(im.Data), imageBegin, flashSize)
		}
		if imageBegin%flashSectorSize != 0 {
			return errors.Errorf("Image starting address (0x%x) is not on flash sector boundary (sector size %d)",
				imageBegin,
				flashSectorSize)
		}
		if imageBegin == 0 && len(im.Data) > 0 {
			if im.Data[0] != espImageMagicByte {
				return errors.Errorf("Invalid magic byte in the first image")
			}
		}
		if ct == esp.ChipESP8266 {
			sysParamsBegin := flashSize - sysParamsAreaSize
			if imageBegin == sysParamsBegin && im.Type == sysParamsPartType {
				// Ok, a sys_params image.
			} else if imageEnd > sysParamsBegin {
				return errors.Errorf("Image 0x%x overlaps with system params area (%d @ 0x%x)",
					imageBegin, sysParamsAreaSize, sysParamsBegin)
			}
		}
		if i > 0 {
			prevImageBegin := int(images[i-1].Addr)
			prevImageEnd := prevImageBegin + len(images[i-1].Data)
			// We traverse the list in order, so a simple check will suffice.
			if prevImageEnd > imageBegin {
				return errors.Errorf("Images 0x%x and 0x%x overlap", prevImageBegin, imageBegin)
			}
		}
	}
	return nil
}

func dedupImages(fc *FlasherClient, images []*image) ([]*image, error) {
	var dedupedImages []*image
	for _, im := range images {
		glog.V(2).Infof("%d @ 0x%x", len(im.Data), im.Addr)
		imAddr := int(im.Addr)
		digests, err := fc.Digest(im.Addr, uint32(len(im.Data)), flashSectorSize)
		if err != nil {
			return nil, errors.Annotatef(err, "%s: failed to compute digest %d @ 0x%x", im.Name, len(im.Data), im.Addr)
		}
		i, offset := 0, 0
		var newImages []*image
		newAddr, newLen, newTotalLen := imAddr, 0, 0
		for offset < len(im.Data) {
			blockLen := flashSectorSize
			if offset+blockLen > len(im.Data) {
				blockLen = len(im.Data) - offset
			}
			digestHex := strings.ToLower(hex.EncodeToString(digests[i]))
			expectedDigest := md5.Sum(im.Data[offset : offset+blockLen])
			expectedDigestHex := strings.ToLower(hex.EncodeToString(expectedDigest[:]))
			glog.V(2).Infof("0x%06x %4d %s %s %t", imAddr+offset, blockLen, expectedDigestHex, digestHex, expectedDigestHex == digestHex)
			if expectedDigestHex == digestHex {
				// Found a matching sector. If we've been building an image,  commit it.
				if newLen > 0 {
					nim := &image{
						Name:         im.Name,
						Type:         im.Type,
						Addr:         uint32(newAddr),
						Data:         im.Data[newAddr-imAddr : newAddr-imAddr+newLen],
						ESP32Encrypt: im.ESP32Encrypt,
					}
					glog.V(2).Infof("%d @ 0x%x", len(nim.Data), nim.Addr)
					newImages = append(newImages, nim)
					newTotalLen += newLen
					newAddr, newLen = 0, 0
				}
			} else {
				// Found a sector that needs to be written. Start a new image or continue the existing one.
				if newLen == 0 {
					newAddr = imAddr + offset
				}
				newLen += blockLen
			}
			offset += blockLen
			i++
		}
		if newLen > 0 {
			nim := &image{
				Name:         im.Name,
				Type:         im.Type,
				Addr:         uint32(newAddr),
				Data:         im.Data[newAddr-imAddr : newAddr-imAddr+newLen],
				ESP32Encrypt: im.ESP32Encrypt,
			}
			newImages = append(newImages, nim)
			glog.V(2).Infof("%d @ %x", len(nim.Data), nim.Addr)
			newTotalLen += newLen
			newAddr, newLen = 0, 0
		}
		glog.V(2).Infof("%d @ 0x%x -> %d", len(im.Data), im.Addr, newTotalLen)
		// There's a price for fragmenting a large image: erasing many individual
		// sectors is slower than erasing a whole block. So unless the difference
		// is substantial, don't bother.
		if newTotalLen < len(im.Data) && (newTotalLen < flashBlockSize || len(im.Data)-newTotalLen >= flashBlockSize) {
			dedupedImages = append(dedupedImages, newImages...)
			common.Reportf("  %7d @ 0x%x -> %d", len(im.Data), im.Addr, newTotalLen)
		} else {
			dedupedImages = append(dedupedImages, im)
		}
	}
	return dedupedImages, nil
}
