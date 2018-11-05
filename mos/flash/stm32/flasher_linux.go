package stm32

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

func GetSTLinkMountForPort(port string) (string, error) {
	port, _ = filepath.EvalSymlinks(port)
	// Find the USB device directory corresponding to the TTY device.
	usbDevDir := ""
	ports, _ := filepath.Glob("/sys/bus/usb/devices/*/*/tty/*")
	for _, p := range ports {
		// /sys/bus/usb/devices/NNN/NNN:1.2/tty/ttyACMx (CDC is interface 1, endpoint 2).
		if filepath.Base(p) == filepath.Base(port) {
			dd1, _ := filepath.Split(p)
			dd2, _ := filepath.Split(filepath.Clean(dd1))
			dd3, _ := filepath.Split(filepath.Clean(dd2))
			usbDevDir = filepath.Clean(dd3) // /sys/bus/usb/devices/NNN
			glog.V(1).Infof("%s -> %s -> %s", port, p, usbDevDir)
		}
	}
	if usbDevDir == "" {
		return "", errors.Errorf("no USB device found for %s", port)
	}
	// Find the block device name associated with this USB device.
	// Expand /sys/bus/usb/devices/NNN/*/*/*/*/block, it will contain the list of devices (only one).
	blockDevs, _ := filepath.Glob(filepath.Join(usbDevDir, "*", "*", "*", "*", "block", "*"))
	if len(blockDevs) == 0 {
		return "", errors.Errorf("no block device found for %s", port)
	}
	dev := filepath.Join("/dev", filepath.Base(blockDevs[0]))
	// Now find mount point for this device.

	glog.V(1).Infof("%s -> %s", port, dev)
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return "", errors.Annotatef(err, "failed to open list of mounts")
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
		return "", errors.Errorf("%s is not mounted", dev)
	}
	return mp, nil
}

func GetSTLinkMounts() ([]string, error) {
	return getSTLinkMountsInDir(filepath.Join("/", "media", os.Getenv("USER")))
}
