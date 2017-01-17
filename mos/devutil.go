package main

import (
	"context"

	"cesanta.com/cloud/cmd/mgos/common/dev"
	"github.com/cesanta/errors"
)

func createDevConn(ctx context.Context) (*dev.DevConn, error) {
	c := dev.Client{Port: *port, Timeout: *timeout, Reconnect: *reconnect}
	devConn, err := c.CreateDevConn(ctx, "serial://"+*port, *reconnect)
	return devConn, errors.Trace(err)
}
