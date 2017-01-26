package main

import (
	"context"

	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
)

func createDevConn(ctx context.Context) (*dev.DevConn, error) {
	c := dev.Client{Port: getPort(), Timeout: *timeout, Reconnect: *reconnect}
	devConn, err := c.CreateDevConn(ctx, "serial://"+getPort(), *reconnect)
	return devConn, errors.Trace(err)
}

func createDevConnWithJunkHandler(
	ctx context.Context, junkHandler func(junk []byte),
) (*dev.DevConn, error) {
	c := dev.Client{Port: getPort(), Timeout: *timeout, Reconnect: *reconnect}
	devConn, err := c.CreateDevConnWithJunkHandler(
		ctx, "serial://"+getPort(), junkHandler, *reconnect,
	)
	return devConn, errors.Trace(err)
}
