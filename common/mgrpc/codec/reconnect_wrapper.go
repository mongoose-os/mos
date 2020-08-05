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
	"fmt"
	"sync"
	"time"

	"github.com/mongoose-os/mos/common/mgrpc/frame"

	"github.com/juju/errors"
	"golang.org/x/net/websocket"
	glog "k8s.io/klog/v2"
)

type ConnectFunc func(addr string) (Codec, error)

type reconnectWrapperCodec struct {
	addr    string
	connect ConnectFunc

	lock        sync.Mutex
	conn        Codec
	connEstd    chan error
	nextAttempt time.Time

	closeNotifier chan struct{}
	closeOnce     sync.Once
}

func NewReconnectWrapperCodec(addr string, connect ConnectFunc) Codec {
	rwc := &reconnectWrapperCodec{
		addr:          addr,
		connect:       connect,
		nextAttempt:   time.Now(),
		connEstd:      make(chan error), // closed when a new connection is established, or an error if permanently fails
		closeNotifier: make(chan struct{}),
	}
	go rwc.maintainConnection()
	return rwc
}

func (rwc *reconnectWrapperCodec) stringLocked() string {
	var connStatus string
	switch {
	case rwc.conn != nil:
		connStatus = "connected"
	case rwc.nextAttempt.After(time.Now()):
		connStatus = fmt.Sprintf("connect in %.2fs", rwc.nextAttempt.Sub(time.Now()).Seconds())
	default:
		connStatus = "connecting..."
	}
	return fmt.Sprintf("[reconnectWrapperCodec to %s; %s]", rwc.addr, connStatus)
}

func (rwc *reconnectWrapperCodec) String() string {
	rwc.lock.Lock()
	defer rwc.lock.Unlock()
	return rwc.stringLocked()
}

func (rwc *reconnectWrapperCodec) maintainConnection() {
	for {
		rwc.lock.Lock()
		conn := rwc.conn
		rwc.lock.Unlock()
		if conn != nil {
			select {
			case <-rwc.closeNotifier:
				glog.V(1).Infof("closed, stopping reconnect thread")
				return
			case <-conn.CloseNotify():
				select {
				case <-rwc.closeNotifier:
					// We are shutting down, don't raise fuss
				default:
					glog.Errorf("%s Connection closed", rwc)
				}
				rwc.lock.Lock()
				rwc.conn = nil
				rwc.connEstd = make(chan error)
				rwc.lock.Unlock()
			}
		}
		glog.V(2).Infof("Next attempt: %s, Now: %s, Diff: %s", rwc.nextAttempt, time.Now(), rwc.nextAttempt.Sub(time.Now()))
		select {
		case <-rwc.closeNotifier:
			glog.V(1).Infof("closed, stopping reconnect thread")
			return
		case <-time.After(rwc.nextAttempt.Sub(time.Now())):
		}

		glog.V(1).Infof("%s connecting", rwc)
		conn, err := rwc.connect(rwc.addr)
		rwc.lock.Lock()
		// TODO(rojer): implement backoff.
		rwc.nextAttempt = time.Now().Add(2 * time.Second)
		if err != nil {
			if errors.Cause(err) == websocket.ErrBadStatus {
				glog.Errorf("%s fatal connection error: %+v", rwc.stringLocked(), err)
				rwc.connEstd <- errors.Trace(err)
			} else {
				glog.Errorf("%s connection error: %+v", rwc.stringLocked(), err)
			}
			rwc.lock.Unlock()
			continue
		}
		rwc.conn = conn
		glog.Infof("%s connected", rwc.stringLocked())
		close(rwc.connEstd)
		rwc.lock.Unlock()
	}
}

func (rwc *reconnectWrapperCodec) getConn(ctx context.Context) (Codec, error) {
	for {
		rwc.lock.Lock()
		conn, connEstd := rwc.conn, rwc.connEstd
		rwc.lock.Unlock()
		if conn != nil {
			return conn, nil
		}
		select {
		case <-ctx.Done():
			return nil, errors.Trace(ctx.Err())
		case err, ok := <-connEstd:
			if ok {
				return nil, errors.Annotatef(err, "fatal connection error, not reconnecting")
			}
		}
	}
}

func (rwc *reconnectWrapperCodec) closeConn() {
	rwc.lock.Lock()
	defer rwc.lock.Unlock()
	if rwc.conn != nil {
		rwc.conn.Close()
		rwc.conn = nil
	}
}

func (rwc *reconnectWrapperCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	for {
		conn, err := rwc.getConn(ctx)
		if err != nil {
			return nil, errors.Trace(err)
		}
		frame, err := conn.Recv(ctx)
		if err != nil {
			glog.V(1).Infof("%s recv error: %s, eof? %v", rwc, err, IsEOF(err))
		}
		switch {
		case err == nil:
			return frame, nil
		case IsEOF(err):
			rwc.closeConn()
			return nil, errors.Trace(err)
		default:
			return nil, errors.Trace(err)
		}
	}
}

func (rwc *reconnectWrapperCodec) Send(ctx context.Context, frame *frame.Frame) error {
	for {
		conn, err := rwc.getConn(ctx)
		if err != nil {
			return errors.Trace(err)
		}
		err = conn.Send(ctx, frame)
		if err != nil {
			glog.V(1).Infof("%s send error: %s", rwc, err)
			rwc.closeConn()
			continue
		}
		return nil
	}
}

func (rwc *reconnectWrapperCodec) Close() {
	rwc.closeOnce.Do(func() {
		close(rwc.closeNotifier)
		if rwc.conn != nil {
			rwc.conn.Close()
		}
	})
}

func (rwc *reconnectWrapperCodec) CloseNotify() <-chan struct{} {
	return rwc.closeNotifier
}

func (rwc *reconnectWrapperCodec) MaxNumFrames() int {
	return -1
}

func (rwc *reconnectWrapperCodec) Info() ConnectionInfo {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel now, we don't want to wait.
	c, err := rwc.getConn(ctx)
	if err != nil {
		return ConnectionInfo{RemoteAddr: rwc.addr}
	}
	return c.Info()
}

func (rwc *reconnectWrapperCodec) SetOptions(opts *Options) error {
	rwc.lock.Lock()
	defer rwc.lock.Unlock()
	if rwc.conn != nil {
		return rwc.conn.SetOptions(opts)
	}
	return errors.Errorf("not connected")
}
