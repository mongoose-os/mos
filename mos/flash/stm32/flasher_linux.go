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
package stm32

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

func GetSTLinkMountForPort(port string) (string, string, error) {
	port, _ = filepath.EvalSymlinks(port)
	// Find the USB device directory corresponding to the TTY device.
	usbDevDir := ""
	ports, err := filepath.Glob("/sys/bus/usb/devices/*/*/tty/*")
	for _, p := range ports {
		// /sys/bus/usb/devices/NNN/NNN:1.2/tty/ttyACMx (CDC is interface 1, endpoint 2).
		if filepath.Base(p) == filepath.Base(port) {
			dd1, _ := filepath.Split(p)
			dd2, _ := filepath.Split(filepath.Clean(dd1))
			dd3, _ := filepath.Split(filepath.Clean(dd2))
			usbDevDir = filepath.Clean(dd3) // /sys/bus/usb/devices/NNN
			glog.V(1).Infof("%s -> %s -> %s", port, p, usbDevDir)
			break
		}
	}
	if usbDevDir == "" {
		return "", "", errors.Errorf("no USB device found for %s", port)
	}
	// Read serial number.
	serialBytes, err := ioutil.ReadFile(filepath.Join(usbDevDir, "serial"))
	serial := strings.TrimSpace(string(serialBytes))
	// Find the block device name associated with this USB device.
	// Expand /sys/bus/usb/devices/NNN/*/*/*/*/block, it will contain the list of devices (only one).
	blockDevs, _ := filepath.Glob(filepath.Join(usbDevDir, "*", "*", "*", "*", "block", "*"))
	if len(blockDevs) == 0 {
		return "", "", errors.Errorf("no block device found for %s", port)
	}
	dev := filepath.Join("/dev", filepath.Base(blockDevs[0]))
	glog.V(1).Infof("%s -> %s serial %s", port, dev, serial)

	// Now find mount point for this device.
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return "", "", errors.Annotatef(err, "failed to open list of mounts")
	}
	defer f.Close()
	mp := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(sc.Text(), " ")
		if parts[0] == dev {
			mp = parts[1]
			break
		}
	}
	if mp == "" {
		return "", "", errors.Errorf("%s is not mounted", dev)
	}
	return mp, serial, nil
}

func GetSTLinkMounts() ([]string, error) {
	return getSTLinkMountsInDir(filepath.Join("/", "media", os.Getenv("USER")))
}
