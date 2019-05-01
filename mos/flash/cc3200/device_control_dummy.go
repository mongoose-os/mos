// +build !linux,!windows,!darwin

package cc3200

import (
	"github.com/mongoose-os/mos/mos/flash/cc32xx"
	"github.com/cesanta/errors"
)

func NewCC3200DeviceControl(port string) (cc32xx.DeviceControl, error) {
	return nil, errors.NotImplementedf("")
}
