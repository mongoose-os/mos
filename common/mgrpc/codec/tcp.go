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
	"net"

	"github.com/juju/errors"
)

type tcpCodec struct {
	conn net.Conn
}

func TCP(conn net.Conn) Codec {
	return newStreamConn(&tcpCodec{
		conn: conn,
	}, false /* addChecksum */, nil)
}

func (c *tcpCodec) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *tcpCodec) WriteWithContext(ctx context.Context, b []byte) (n int, err error) {
	/* TODO(dfrank): use ctx */
	return c.conn.Write(b)
}

func (c *tcpCodec) Close() error {
	return c.conn.Close()
}

func (c *tcpCodec) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func (c *tcpCodec) PreprocessFrame(frameData []byte) (bool, error) {
	return false, nil
}

func (c *tcpCodec) SetOptions(opts *Options) error {
	return errors.NotImplementedf("SetOptions")
}
