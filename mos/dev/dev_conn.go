package dev

import (
	"context"

	"cesanta.com/clubby"
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
	Instance    *clubby.Instance
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
	dc.Instance.Dispatcher.RemoveDefault(ctx, nil)
	dc.Instance = nil
	return nil
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
	if dc.Instance != nil {
		return nil
	}

	dc.JunkHandler = junkHandler
	dc.Reconnect = reconnect

	// create a separate clubby instance for talking to the device over uart,
	// since we're going to use an empty destination id, so that we can't add
	// a channel to the existing clubby instance for that
	dci := clubby.New(ctx, "mos", "")
	opts := []clubby.ConnectOption{
		clubby.SendHello(false),
		clubby.ConnectTo(dc.ClubbyAddr),
		clubby.DefaultRoute(),
		clubby.JunkHandler(junkHandler),
		clubby.Reconnect(reconnect),
	}

	// Connection to the device also includes sending of '\x04"""' until we
	// receive '"""' in reply, so it needs to be guarded by timeout.
	err := dc.c.RunWithTimeout(ctx, func(ctx context.Context) error {
		// we have to use `dummy_id` because otherwise Connect complains, but this
		// id won't be actually used since we're going to use empty destination
		// instead
		err := dci.Connect(ctx, "dummy_id", opts...)
		return errors.Annotatef(err, "error connecting to the device via %s", dc.ClubbyAddr)
	})
	if err != nil {
		return errors.Trace(err)
	}
	dc.Instance = dci
	dc.CConf = fwconfig.NewClient(dci, debugDevId)
	dc.CVars = fwvars.NewClient(dci, debugDevId)
	dc.CFilesystem = fwfilesystem.NewClient(dci, debugDevId)
	return nil
}
