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
package codec

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"golang.org/x/net/websocket"
	glog "k8s.io/klog/v2"
)

const (
	WSProtocol = "clubby.cesanta.com"
)

func jsonMarshal(v interface{}) ([]byte, byte, error) {
	if _, ok := v.(*frame.Frame); !ok {
		return nil, websocket.TextFrame, errors.Errorf("only clubby frames are supported, got %T", v)
	}
	b, err := frame.MarshalJSON(v.(*frame.Frame))
	return b, websocket.TextFrame, err
}

func jsonUnmarshal(data []byte, payloadType byte, v interface{}) error {
	f12, ok := v.(*frame.Frame)
	if !ok {
		return errors.Errorf("only clubby frames are supported, got %T", v)
	}
	f12.SizeHint = len(data)
	if payloadType != websocket.TextFrame && payloadType != websocket.BinaryFrame {
		return errors.Errorf("unknown frame type: %d", payloadType)
	}
	return errors.Trace(json.Unmarshal(data, f12))
}

func WebSocket(conn *websocket.Conn) Codec {
	r := &wsCodec{
		closeNotify: make(chan struct{}),
		conn:        conn,
		codec:       websocket.Codec{Marshal: jsonMarshal, Unmarshal: jsonUnmarshal},
	}
	return r
}

type wsCodec struct {
	closeNotify chan struct{}
	conn        *websocket.Conn
	closeOnce   sync.Once
	codec       websocket.Codec
}

func (c *wsCodec) String() string {
	addr := "unknown address"
	if c.conn.Request() != nil && c.conn.Request().RemoteAddr != "" {
		addr = c.conn.Request().RemoteAddr
	}
	return fmt.Sprintf("[wsCodec from %s]", addr)
}

func (c *wsCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	var f12 frame.Frame
	if err := c.codec.Receive(c.conn, &f12); err != nil {
		glog.V(2).Infof("%s Recv(): %s", c, err)
		c.Close()
		return nil, errors.Trace(err)
	}
	return &f12, nil
}

func (c *wsCodec) Send(ctx context.Context, f12 *frame.Frame) error {
	return errors.Trace(c.codec.Send(c.conn, f12))
}

func (c *wsCodec) Close() {
	c.closeOnce.Do(func() {
		glog.V(1).Infof("Closing %s", c)
		close(c.closeNotify)
		c.conn.Close()
	})
}

func (c *wsCodec) CloseNotify() <-chan struct{} {
	return c.closeNotify
}

func (c *wsCodec) MaxNumFrames() int {
	return -1
}

func (c *wsCodec) Info() ConnectionInfo {

	req := c.conn.Request()

	// For server connections, request is not nil, and for client connection
	// it's nil, so we have different logic here
	if req != nil {
		// Server websocket connection

		r := ConnectionInfo{
			IsConnected: true,
			TLS:         req.TLS != nil,
			RemoteAddr:  req.RemoteAddr,
		}
		if r.TLS {
			r.PeerCertificates = c.conn.Request().TLS.PeerCertificates
		}
		return r
	} else {
		// Client websocket connection

		r := ConnectionInfo{
			IsConnected: true,
			TLS:         c.conn.Config().TlsConfig != nil,
			RemoteAddr:  c.conn.RemoteAddr().String(),
		}
		// TODO(dfrank): set r.PeerCertificates
		return r
	}
}

func (c *wsCodec) SetOptions(opts *Options) error {
	return errors.NotImplementedf("SetOptions")
}
