package dev

import (
	"context"

	"cesanta.com/common/go/mgrpc"
	"cesanta.com/common/go/ourjson"
	fwconfig "cesanta.com/fw/defs/config"
	fwfilesystem "cesanta.com/fw/defs/fs"
	fwvars "cesanta.com/fw/defs/vars"
	"github.com/cesanta/errors"
)

const (
	// we use empty destination so that device will receive the frame over uart
	// and handle it
	debugDevId = ""
)

type DevConn struct {
	c           *Client
	ClubbyAddr  string
	RPC         *mgrpc.MgRPC
	Dest        string
	JunkHandler func(junk []byte)
	Reconnect   bool

	CConf       fwconfig.Service
	CVars       fwvars.Service
	CFilesystem fwfilesystem.Service
}

// CreateDevConn creates a direct connection to the device at a given address,
// which could be e.g. "serial:///dev/ttyUSB0", "serial://COM7",
// "tcp://192.168.0.10", etc.
func (c *Client) CreateDevConn(
	ctx context.Context, clubbyAddr string, reconnect bool,
) (*DevConn, error) {

	dc := &DevConn{c: c, ClubbyAddr: clubbyAddr, Dest: debugDevId}

	err := dc.Connect(ctx, reconnect)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return dc, nil
}

func (c *Client) CreateDevConnWithJunkHandler(
	ctx context.Context, clubbyAddr string, junkHandler func(junk []byte), reconnect bool,
) (*DevConn, error) {

	dc := &DevConn{c: c, ClubbyAddr: clubbyAddr, Dest: debugDevId}

	err := dc.ConnectWithJunkHandler(ctx, junkHandler, reconnect)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return dc, nil
}

func (dc *DevConn) GetConfig(ctx context.Context) (*DevConf, error) {
	devConfRaw, err := dc.CConf.Get(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var devConf DevConf

	err = devConfRaw.UnmarshalInto(&devConf.data)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &devConf, nil
	return nil, nil
}

func (dc *DevConn) SetConfig(ctx context.Context, devConf *DevConf) error {
	err := dc.CConf.Set(ctx, &fwconfig.SetArgs{
		Config: ourjson.DelayMarshaling(devConf.data),
	})
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (dc *DevConn) Disconnect(ctx context.Context) error {
	return dc.RPC.Disconnect(ctx)
}

func (dc *DevConn) Connect(ctx context.Context, reconnect bool) error {
	if dc.JunkHandler == nil {
		dc.JunkHandler = func(junk []byte) {}
	}
	return dc.ConnectWithJunkHandler(ctx, dc.JunkHandler, reconnect)
}

func (dc *DevConn) ConnectWithJunkHandler(
	ctx context.Context, junkHandler func(junk []byte), reconnect bool,
) error {
	var err error

	if dc.RPC != nil {
		return nil
	}

	dc.JunkHandler = junkHandler
	dc.Reconnect = reconnect

	dc.RPC, err = mgrpc.New(ctx, "mos", dc.ClubbyAddr, junkHandler, reconnect)
	if err != nil {
		return errors.Trace(err)
	}

	dc.CConf = fwconfig.NewClient(dc.RPC, debugDevId)
	dc.CVars = fwvars.NewClient(dc.RPC, debugDevId)
	dc.CFilesystem = fwfilesystem.NewClient(dc.RPC, debugDevId)
	return nil
}
