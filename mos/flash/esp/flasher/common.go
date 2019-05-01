package flasher

import (
	"cesanta.com/mos/flash/esp"
	"cesanta.com/mos/flash/esp/rom_client"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

const (
	defaultFlashMode = "dio"
	defaultFlashFreq = "80m"
)

type cfResult struct {
	rc          *rom_client.ROMClient
	fc          *FlasherClient
	flashParams flashParams
}

func ConnectToFlasherClient(ct esp.ChipType, opts *esp.FlashOpts) (*cfResult, error) {
	var err error
	r := &cfResult{}

	if opts.FlasherBaudRate < 0 || opts.FlasherBaudRate > 4000000 {
		return nil, errors.Errorf("invalid flashing baud rate (%d)", opts.FlasherBaudRate)
	}

	if err = r.flashParams.ParseString(ct, opts.FlashParams); err != nil {
		return nil, errors.Annotatef(err, "invalid flash params (%q)", opts.FlashParams)
	}

	ownROMClient := false
	defer func() {
		if ownROMClient {
			r.rc.Disconnect()
		}
	}()
	flasherBaudRate := opts.FlasherBaudRate
	for {
		r.rc, err = rom_client.ConnectToROM(ct, opts)
		if err != nil {
			return nil, errors.Annotatef(
				err,
				"Failed to talk to bootloader.\nSee "+
					"https://github.com/espressif/esptool/wiki/ESP8266-Boot-Mode-Selection\n"+
					"for wiring instructions or pull GPIO0 low and reset.",
			)
		}
		ownROMClient = true

		r.fc, err = NewFlasherClient(ct, r.rc, opts.ROMBaudRate, flasherBaudRate)
		if err == nil {
			break
		}
		if flasherBaudRate != 0 {
			glog.Errorf("failed to run flasher @ %d, falling back to ROM baud rate...", flasherBaudRate)
			r.rc.Disconnect()
			ownROMClient = false
			flasherBaudRate = 0
		} else {
			return nil, errors.Annotatef(err, "failed to run flasher")
		}
	}
	if r.flashParams.Size() <= 0 || r.flashParams.Mode() == "" {
		mfg, flashSize, err := detectFlashSize(r.fc)
		if err != nil {
			return nil, errors.Annotatef(err, "flash size is not specified and could not be detected")
		}
		if err = r.flashParams.SetSize(flashSize); err != nil {
			return nil, errors.Annotatef(err, "invalid flash size detected")
		}
		if r.flashParams.Mode() == "" {
			if ct == esp.ChipESP8266 && mfg == 0x51 && flashSize == 1048576 {
				// ESP8285's built-in flash requires dout mode.
				r.flashParams.SetMode("dout")
			} else {
				r.flashParams.SetMode(defaultFlashMode)
			}
		}
	}
	if r.flashParams.Freq() == "" {
		r.flashParams.SetFreq(defaultFlashFreq)
	}
	ownROMClient = false
	return r, nil
}

func detectFlashSize(fc *FlasherClient) (int, int, error) {
	chipID, err := fc.GetFlashChipID()
	if err != nil {
		return 0, 0, errors.Annotatef(err, "failed to get flash chip id")
	}
	// Parse the JEDEC ID.
	mfg := int((chipID >> 16) & 0xff)
	sizeExp := (chipID & 0xff)
	glog.V(2).Infof("Flash chip ID: 0x%08x, mfg: 0x%02x, sizeExp: %d", chipID, mfg, sizeExp)
	if mfg == 0 || sizeExp < 19 || sizeExp > 32 {
		return 0, 0, errors.Errorf("invalid chip id: 0x%08x", chipID)
	}
	// Capacity is the power of two.
	return mfg, (1 << sizeExp), nil
}
