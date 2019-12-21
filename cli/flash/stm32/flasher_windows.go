/*
 * Copyright (c) 2014-2018 Cesanta Software Limited
 * All rights reserved
 */

package stm32

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/ourutil"
	"golang.org/x/sys/windows"
)

// GetSTLinkMounts enumerated drives and finds ones that have
// a name starting with one of the known prefixes.
func GetSTLinkMounts() ([]string, error) {
	drivesBitMask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, errors.Annotatef(err, "GetLogicalDrives")
	}
	var stmDrives []string
	glog.Infof("drives: %08x", drivesBitMask)
	for i := uint32(0); i < 32; i++ {
		if drivesBitMask&(uint32(1)<<i) == 0 {
			continue
		}
		drive := fmt.Sprintf("%c:\\", 65+i)
		label, fsType, serial, err := getDriveInfo(drive)
		if err != nil {
			glog.Infof("%s %s", drive, err)
			continue
		}
		glog.Infof("%s %s %s %08x", drive, label, fsType, serial)
		if fsType != "FAT" {
			continue
		}
		for _, p := range stlinkDevPrefixes {
			if strings.HasPrefix(label, p) {
				ourutil.Reportf("Found STLink drive: %s", drive)
				stmDrives = append(stmDrives, drive)
			}
		}
	}

	return stmDrives, nil
}

func getDriveInfo(drive string) (string, string, uint32, error) {
	volName := make([]uint16, windows.MAX_PATH+1)
	fsName := make([]uint16, windows.MAX_PATH+1)
	var volSerial, maxCompLen, fsFlags uint32
	if err := windows.GetVolumeInformation(
		syscall.StringToUTF16Ptr(drive),
		&volName[0], windows.MAX_PATH,
		&volSerial, &maxCompLen, &fsFlags,
		&fsName[0], windows.MAX_PATH); err != nil {
		return "", "", 0, errors.Annotatef(err, "GetVolumeInformation")
	}

	return syscall.UTF16ToString(volName), syscall.UTF16ToString(fsName), volSerial, nil
}

func GetSTLinkMountForPort(port string) (string, string, error) {
	// TODO(rojer)
	return "", "", errors.NotImplementedf("GetSTLinkMountForPort")
}
