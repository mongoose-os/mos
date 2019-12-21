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
// +build !no_libudev

package cc3220

import (
	"time"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/flash/cc3220/xds110"
	"github.com/mongoose-os/mos/cli/flash/cc32xx"
	"github.com/mongoose-os/mos/cli/flash/common"
)

type xds110DeviceControl struct {
	xc *xds110.XDS110Client
}

func NewCC3220DeviceControl(port string) (cc32xx.DeviceControl, error) {
	// Try to get serial number of this device but proceed without it in case of failure.
	sn, _ := cc32xx.GetUSBSerialNumberForPort(port)
	common.Reportf("Using XDS110 debug probe to control the device...")
	xc, err := xds110.NewXDS110Client(sn)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to open XDS110")
	}
	vi, err := xc.GetVersionInfo()
	for i := 0; err != nil && i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		vi, err = xc.GetVersionInfo()
	}
	if err != nil {
		return nil, errors.Annotatef(err, "failed to get version")
	}
	common.Reportf("  XDS110 %d.%d.%d.%d HW %d S/N %s", vi.V1, vi.V2, vi.V3, vi.V4, vi.HWVersion, xc.GetSerialNumber())
	err = xc.Connect()
	if err != nil {
		return nil, errors.Annotatef(err, "failed to enable the probe")
	}
	return &xds110DeviceControl{xc: xc}, nil
}

func (dc *xds110DeviceControl) EnterBootLoader() error {
	if err := dc.xc.SetSRST(true); err != nil {
		return errors.Trace(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := dc.xc.SetSRST(false); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (dc *xds110DeviceControl) BootFirmware() error {
	return dc.EnterBootLoader()
}

func (dc *xds110DeviceControl) Close() {
	dc.xc.Disconnect()
	dc.xc.Close()
}
