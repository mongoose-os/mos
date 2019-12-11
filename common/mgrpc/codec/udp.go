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
	"net"

	"github.com/juju/errors"

	"github.com/mongoose-os/mos/common/mgrpc/frame"
)

const (
	UDPURLScheme = "udp"
)

type UDPCodecOptions struct {
}

type udpCodec struct {
	conn net.Conn
}

func UDP(addr string) Codec {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil
	}
	return &udpCodec{conn: conn}
}

func (c *udpCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	buf := make([]byte, 10000)
	readLen, err := c.conn.Read(buf)
	if err != nil {
		c.Close()
		return nil, errors.Trace(err)
	}
	var f frame.Frame
	if err = json.Unmarshal(buf[:readLen], &f); err != nil {
		return nil, errors.Trace(err)
	}
	return &f, nil
}

func (c *udpCodec) Send(ctx context.Context, f *frame.Frame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = c.conn.Write(b)
	return errors.Trace(err)
}

func (c *udpCodec) Close() {
	c.conn.Close()
}

func (c *udpCodec) CloseNotify() <-chan struct{} {
	return nil
}

func (c *udpCodec) MaxNumFrames() int {
	return -1
}

func (c *udpCodec) Info() ConnectionInfo {
	return ConnectionInfo{
		IsConnected: true,
		TLS:         false,
		RemoteAddr:  c.conn.RemoteAddr().String(),
	}
}

func (c *udpCodec) SetOptions(opts *Options) error {
	return errors.NotImplementedf("SetOptions")
}
