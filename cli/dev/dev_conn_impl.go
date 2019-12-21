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
package dev

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"time"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/mgrpc"
	"github.com/mongoose-os/mos/common/mgrpc/codec"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/mongoose-os/mos/cli/rpccreds"
	flag "github.com/spf13/pflag"
)

const (
	// we use empty destination so that device will receive the frame over uart
	// and handle it
	debugDevId = ""
)

var (
	mgrpcCompatArgsFlag = flag.Bool("mgrpc-compat-args", false, "Use args field in the RPC frame, for compatibility with older firmware")
)

type MosDevConn struct {
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
) (*MosDevConn, error) {

	dc := &MosDevConn{c: c, ConnectAddr: connectAddr, Dest: debugDevId}

	err := dc.Connect(ctx, reconnect)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return dc, nil
}

func (c *Client) CreateDevConnWithOpts(ctx context.Context, connectAddr string, reconnect bool, tlsConfig *tls.Config, codecOpts *codec.Options) (*MosDevConn, error) {

	dc := &MosDevConn{c: c, ConnectAddr: connectAddr, Dest: debugDevId}

	err := dc.ConnectWithOpts(ctx, reconnect, tlsConfig, codecOpts)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return dc, nil
}

func (dc *MosDevConn) Disconnect(ctx context.Context) error {
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

func (dc *MosDevConn) IsConnected() bool {
	return dc.RPC != nil && dc.RPC.IsConnected()
}

func (dc *MosDevConn) Connect(ctx context.Context, reconnect bool) error {
	return dc.ConnectWithOpts(ctx, reconnect, nil, nil)
}

func (dc *MosDevConn) ConnectWithOpts(ctx context.Context, reconnect bool, tlsConfig *tls.Config, codecOpts *codec.Options) error {
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

func (dc *MosDevConn) GetTimeout() time.Duration {
	return dc.c.Timeout
}

func isJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

func (dc *MosDevConn) CallRaw(ctx context.Context, method string, args interface{}) (json.RawMessage, error) {
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
		cmd.Args.UnmarshalJSON([]byte(argsJSON))
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

func (dc *MosDevConn) CallB(ctx context.Context, method string, args interface{}) ([]byte, error) {
	respRaw, err := dc.CallRaw(ctx, method, args)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Ignoring errors here, cause response could be empty which is a success
	res, _ := json.MarshalIndent(respRaw, "", "  ")
	return res, nil
}

func (dc *MosDevConn) Call(ctx context.Context, method string, args interface{}, resp interface{}) error {
	respRaw, err := dc.CallRaw(ctx, method, args)
	if err != nil {
		return errors.Trace(err)
	}
	if resp != nil {
		return json.Unmarshal(respRaw, resp)
	}
	return nil
}
