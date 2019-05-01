// +build !no_libudev

package common

import (
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	"github.com/google/gousb"
)

// OpenUSBDevice opens a USB device with specified VID, PID and (optionally) serial number.
// If serial number is empty, it is not checked.
// If multiple devices match the criteria, one of them will be returned.
func OpenUSBDevice(vid, pid gousb.ID, serial string) (*gousb.Context, *gousb.Device, error) {
	uctx := gousb.NewContext()
	devs, err := uctx.OpenDevices(func(dd *gousb.DeviceDesc) bool {
		result := (dd.Vendor == vid && dd.Product == pid)
		glog.V(1).Infof("Dev %+v", dd)
		return result
	})
	// OpenDevices may fail overall but still return results. Only fail if no devices were returned.
	if err != nil && len(devs) == 0 {
		uctx.Close()
		return nil, nil, errors.Annotatef(err, "failed to enumerate USB devices")
	}
	var res *gousb.Device
	for _, dev := range devs {
		if res != nil {
			// Found one already
			dev.Close()
			continue
		}
		sn, _ := dev.SerialNumber()
		glog.V(1).Infof("Dev %+v sn '%s'", dev, sn)
		if serial == "" || sn == serial {
			res = dev
		} else {
			dev.Close()
		}
	}
	if res == nil {
		sp := ""
		if serial != "" {
			sp = "/"
		}
		uctx.Close()
		return nil, nil, errors.Errorf("No device matching %s:%s%s%s found", vid, pid, sp, serial)
	}
	return uctx, res, nil
}
