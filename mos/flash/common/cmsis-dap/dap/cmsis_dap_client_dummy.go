// +build no_libudev

package dap

import (
	"context"

	"github.com/cesanta/errors"
)

func NewClient(ctx context.Context, vid, pid uint16, serial string, intf, epIn, epOut int) (DAPClient, error) {
	return nil, errors.Errorf("not supported in this build")
}
