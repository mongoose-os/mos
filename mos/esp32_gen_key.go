// +build !noflash

package main

import (
	"context"
	"crypto/rand"
	"io/ioutil"
	"math/big"
	"os"
	"strings"

	moscommon "cesanta.com/mos/common"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/flash/esp32"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

var (
	esp32ProtectKey = flag.Bool("esp32-protect-key", true,
		"Write and read-protect the key inside the device.")
	esp32EnableFlashEncryption = flag.Bool("esp32-enable-flash-encryption", false,
		"Enable flash encryption. This sets a typical set of eFuse options used with flash encryption.")
)

func esp32GenKey(ctx context.Context, devConn dev.DevConn) error {
	if len(flag.Args()) < 2 {
		return errors.Errorf("key slot is required")
	}
	keySlot := flag.Args()[1]
	outFile := espFlashOpts.ESP32EncryptionKeyFile
	if len(flag.Args()) > 2 {
		outFile = flag.Args()[2]
	}

	rrw, err := getRRW()
	if err != nil {
		return errors.Trace(err)
	}
	defer rrw.Disconnect()

	blocks, _, fusesByName, err := esp32.ReadFuses(rrw)
	if err != nil {
		return errors.Annotatef(err, "failed to read eFuses")
	}

	mac := fusesByName[esp32.MACAddressFuseName].MACAddressString()
	reportf("Device MAC address: %s", mac)

	kf := fusesByName[keySlot]
	if kf == nil || !kf.IsKey() {
		return errors.Errorf("invalid key slot %s", keySlot)
	}

	v, err := kf.Value(false)
	if !kf.IsReadable() || !kf.IsWritable() || err != nil || v.Cmp(big.NewInt(0)) != 0 {
		return errors.Errorf("%s is already set", keySlot)
	}

	keyLen := 32
	kcs := esp32.GetKeyEncodingScheme(fusesByName)

	switch kcs {
	case esp32.KeyEncodingSchemeNone:
		keyLen = 32
	case esp32.KeyEncodingScheme34:
		keyLen = 24
	case esp32.KeyEncodingSchemeRepeat:
		keyLen = 16
	}

	key := make([]byte, keyLen)
	_, err = rand.Read(key)
	if err != nil {
		return errors.Annotatef(err, "failed to generate key")
	}

	if err = kf.SetKeyValue(key, kcs); err != nil {
		return errors.Annotatef(err, "failed to set key value")
	}

	toPrint := []*esp32.Fuse{kf}

	if *esp32ProtectKey {
		kf.SetReadDisable()
		kf.SetWriteDisable()
		toPrint = append(toPrint, fusesByName[esp32.ReadDisableFuseName])
		toPrint = append(toPrint, fusesByName[esp32.WriteDisableFuseName])
	}

	if *esp32EnableFlashEncryption {
		for _, e := range []struct {
			name  string
			value int64
		}{
			{"flash_crypt_cnt", 1},
			{"flash_crypt_cnt.WD", 1}, // write-protect the counter so encryption cannot be disabled.
			{"JTAG_disable", 1},
			{"download_dis_encrypt", 1},
			{"download_dis_decrypt", 1},
			{"download_dis_cache", 1},
			{"flash_crypt_config", 0xf},
		} {
			f := fusesByName[e.name]
			if err = f.SetValue(big.NewInt(e.value)); err != nil {
				return errors.Annotatef(err, "unable to set %s = %d", e.name, e.value)
			}
			toPrint = append(toPrint, f)
		}
	}

	reportf("")

	for _, f := range toPrint {
		if f.HasDiffs() {
			reportf("%s", f)
		}
	}

	reportf("")
	if outFile != "" {
		if outFile != "-" {
			outFile = moscommon.ExpandPlaceholders(outFile, "?", strings.ToUpper(strings.Replace(mac, ":", "", -1)))
			if _, err := os.Stat(outFile); err == nil {
				return errors.Errorf("key output file %q already exists", outFile)
			}
		}
		if !*dryRun {
			if outFile == "-" {
				os.Stdout.Write(key)
			} else {
				if err = ioutil.WriteFile(outFile, key, 0400); err != nil {
					return errors.Annotatef(err, "failed to write key")
				}
				reportf("Wrote key to %s", outFile)
			}
		} else {
			reportf("Key output file: %s", outFile)
		}
	} else {
		reportf("Warning: not saving key")
	}

	reportf("")
	for i, b := range blocks {
		if err := b.WriteDiffs(); err != nil {
			return errors.Annotatef(err, "failed to write fuse block %d", i)
		}
	}

	if !*dryRun {
		reportf("Programming eFuses...")
		err = esp32.ProgramFuses(rrw)
		if err == nil {
			reportf("Success")
		}
	} else {
		reportf("Not applying changes, set --dry-run=false to burn the fuses.\r\n" +
			"Warning: this is an irreversible one time operation, eFuses cannot be unset.")
	}

	return nil
}
