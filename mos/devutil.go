package main

import (
	"context"
	"strings"

	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
)

func createDevConn(ctx context.Context) (*dev.DevConn, error) {
	port, err := getPort()
	if err != nil {
		return nil, errors.Trace(err)
	}
	c := dev.Client{Port: port, Timeout: *timeout, Reconnect: *reconnect}
	devConn, err := c.CreateDevConn(ctx, "serial://"+port, *reconnect)
	return devConn, errors.Trace(err)
}

func createDevConnWithJunkHandler(
	ctx context.Context, junkHandler func(junk []byte),
) (*dev.DevConn, error) {
	port, err := getPort()
	if err != nil {
		return nil, errors.Trace(err)
	}
	c := dev.Client{Port: port, Timeout: *timeout, Reconnect: *reconnect}
	prefix := "serial://"
	if strings.Index(port, "://") > 0 {
		prefix = ""
	}
	addr := prefix + port
	devConn, err := c.CreateDevConnWithJunkHandler(ctx, addr, junkHandler, *reconnect)
	return devConn, errors.Trace(err)
}
