package dev

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"time"

	"cesanta.com/common/go/mgrpc"
	"cesanta.com/common/go/mgrpc/codec"
	"cesanta.com/common/go/mgrpc/frame"
	"cesanta.com/common/go/ourjson"
	"cesanta.com/mos/rpccreds"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

const (
	// we use empty destination so that device will receive the frame over uart
	// and handle it
	debugDevId = ""

	confOpAttempts = 3
)

var (
	mgrpcCompatArgsFlag = flag.Bool("mgrpc-compat-args", false, "Use args field in the RPC frame, for compatibility with older firmware")
)

type DevConn struct {
	c           *Client
	ConnectAddr string
	RPC         mgrpc.MgRPC
	Dest        string
	Reconnect   bool
	codecOpts   codec.Options
}

// CreateDevConn creates a direct connection to the device at a given address,
// which could be e.g. "serial:///dev/ttyUSB0", "serial://COM7",
// "tcp://192.168.0.10", etc.
func (c *Client) CreateDevConn(
	ctx context.Context, connectAddr string, reconnect bool,
) (*DevConn, error) {

	dc := &DevConn{c: c, ConnectAddr: connectAddr, Dest: debugDevId}

	err := dc.Connect(ctx, reconnect)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return dc, nil
}

func (c *Client) CreateDevConnWithOpts(ctx context.Context, connectAddr string, reconnect bool, tlsConfig *tls.Config, codecOpts *codec.Options) (*DevConn, error) {

	dc := &DevConn{c: c, ConnectAddr: connectAddr, Dest: debugDevId}

	err := dc.ConnectWithOpts(ctx, reconnect, tlsConfig, codecOpts)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return dc, nil
}

func (dc *DevConn) GetConfig(ctx context.Context) (*DevConf, error) {
	var devConf DevConf
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		if err := dc.Call(ctx2, "Config.Get", nil, &devConf.data); err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			return nil, errors.Trace(err)
		}
		break
	}

	return &devConf, nil
}

type ConfigSetArg struct {
	Config map[string]interface{} `json:"config,omitempty"`
}

func (dc *DevConn) SetConfig(ctx context.Context, devConf *DevConf) error {
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		if err := dc.Call(ctx2, "Config.Set", &ConfigSetArg{
			Config: devConf.diff,
		}, nil); err != nil {
			attempts -= 1
			if attempts > 0 {
				glog.Warningf("Error: %s", err)
				continue
			}
			return errors.Trace(err)
		}
		break
	}

	return nil
}

type GetInfoResultWifi struct {
	SSSID  *string `json:"ssid,omitempty"`
	StaIP  *string `json:"sta_ip,omitempty"`
	APIP   *string `json:"ap_ip,omitempty"`
	Status *string `json:"status,omitempty"`
}

type GetInfoResult struct {
	App        *string            `json:"app,omitempty"`
	Arch       *string            `json:"arch,omitempty"`
	Fs_free    *int64             `json:"fs_free,omitempty"`
	Fs_size    *int64             `json:"fs_size,omitempty"`
	Fw_id      *string            `json:"fw_id,omitempty"`
	Fw_version *string            `json:"fw_version,omitempty"`
	Mac        *string            `json:"mac,omitempty"`
	RAMFree    *int64             `json:"ram_free,omitempty"`
	RAMMinFree *int64             `json:"ram_min_free,omitempty"`
	RAMSize    *int64             `json:"ram_size,omitempty"`
	Uptime     *int64             `json:"uptime,omitempty"`
	Wifi       *GetInfoResultWifi `json:"wifi,omitempty"`
}

func (dc *DevConn) GetInfo(ctx context.Context) (*GetInfoResult, error) {
	var r GetInfoResult
	attempts := confOpAttempts
	for {
		ctx2, cancel := context.WithTimeout(ctx, dc.GetTimeout())
		defer cancel()
		var err error
		if err = dc.Call(ctx2, "Sys.GetInfo", nil, &r); err == nil {
			return &r, nil
		}
		attempts -= 1
		if attempts > 0 {
			glog.Warningf("Error: %s", err)
			continue
		}
		return nil, errors.Trace(err)
	}
}

func (dc *DevConn) Disconnect(ctx context.Context) error {
	glog.V(2).Infof("Disconnecting from %s", dc.ConnectAddr)
	err := dc.RPC.Disconnect(ctx)
	// On Windows, closing a port and immediately opening it back is not going to
	// work, so here we just use a random 500ms timeout which seems to solve the
	// problem.
	//
	// Just in case though, we sleep not only on Windows, but on all platforms.
	time.Sleep(500 * time.Millisecond)

	// We need to set RPC to nil, in order for the subsequent call to Connect()
	// to work
	dc.RPC = nil
	return err
}

func (dc *DevConn) IsConnected() bool {
	return dc.RPC != nil && dc.RPC.IsConnected()
}

func (dc *DevConn) Connect(ctx context.Context, reconnect bool) error {
	return dc.ConnectWithOpts(ctx, reconnect, nil, nil)
}

func (dc *DevConn) ConnectWithOpts(ctx context.Context, reconnect bool, tlsConfig *tls.Config, codecOpts *codec.Options) error {
	var err error

	if dc.RPC != nil {
		return nil
	}

	if codecOpts != nil {
		dc.codecOpts = *codecOpts
	}

	opts := []mgrpc.ConnectOption{
		mgrpc.LocalID("mos"),
		mgrpc.Reconnect(reconnect),
		mgrpc.TlsConfig(tlsConfig),
		mgrpc.CompatArgs(*mgrpcCompatArgsFlag),
		mgrpc.CodecOptions(dc.codecOpts),
	}

	dc.RPC, err = mgrpc.New(ctx, dc.ConnectAddr, opts...)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (dc *DevConn) GetTimeout() time.Duration {
	return dc.c.Timeout
}

func isJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

func (dc *DevConn) CallRaw(ctx context.Context, method string, args interface{}) (ourjson.RawMessage, error) {
	argsJSON, ok := args.(string)
	if !ok {
		if args != nil {
			b, err := json.Marshal(args)
			if err != nil {
				return nil, errors.Annotatef(err, "failed to serialize args")
			}
			argsJSON = string(b)
		} else {
			argsJSON = ""
		}
	} else {
		if argsJSON != "" && !isJSON(argsJSON) {
			return nil, errors.Errorf("Args [%s] is not a valid JSON string", args)
		}
	}

	cmd := &frame.Command{Cmd: method}
	if argsJSON != "" {
		cmd.Args = ourjson.RawJSON([]byte(argsJSON))
	}

	resp, err := dc.RPC.Call(ctx, dc.Dest, cmd, rpccreds.GetRPCCreds)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if resp.Status != 0 {
		return nil, errors.Errorf("remote error %d: %s", resp.Status, resp.StatusMsg)
	}

	return resp.Response, nil
}

func (dc *DevConn) CallB(ctx context.Context, method string, args interface{}) ([]byte, error) {
	respRaw, err := dc.CallRaw(ctx, method, args)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Ignoring errors here, cause response could be empty which is a success
	res, _ := json.MarshalIndent(respRaw, "", "  ")
	return res, nil
}

func (dc *DevConn) Call(ctx context.Context, method string, args interface{}, resp interface{}) error {
	respRaw, err := dc.CallRaw(ctx, method, args)
	if err != nil {
		return errors.Trace(err)
	}
	if resp != nil {
		return respRaw.UnmarshalInto(resp)
	}
	return nil
}
