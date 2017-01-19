package main

import (
	"context"

	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
)

func createDevConn(ctx context.Context) (*dev.DevConn, error) {
	c := dev.Client{Port: *port, Timeout: *timeout, Reconnect: *reconnect}
	devConn, err := c.CreateDevConn(ctx, "serial://"+*port, *reconnect)
	return devConn, errors.Trace(err)
}

func createDevConnWithJunkHandler(
	ctx context.Context, junkHandler func(junk []byte),
) (*dev.DevConn, error) {
	c := dev.Client{Port: *port, Timeout: *timeout, Reconnect: *reconnect}
	devConn, err := c.CreateDevConnWithJunkHandler(
		ctx, "serial://"+*port, junkHandler, *reconnect,
	)
	return devConn, errors.Trace(err)
}
