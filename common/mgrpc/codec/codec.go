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
	"crypto/x509"
	"io"
	"runtime"
	"strings"

	"github.com/juju/errors"

	"github.com/mongoose-os/mos/common/mgrpc/frame"
)

// Codec represents a transport for clubby frames.
type Codec interface {
	// Recv returns the next incoming frame.
	Recv(context.Context) (*frame.Frame, error)
	// Send sends the frame to the remote peer.
	Send(context.Context, *frame.Frame) error
	// Close closes the channel.
	Close()
	// CloseNotify() returns a channel that will be closed once the underlying channel has been closed.
	CloseNotify() <-chan struct{}
	// Can accept this many outgoing frames. 0 means no frames can be sent, values < 0 mean no limit.
	MaxNumFrames() int
	// Info() returns information about underlying connection.
	Info() ConnectionInfo
	// SetOptions() adjusts codec options.
	SetOptions(opts *Options) error
}

type Options struct {
	AzureDM AzureDMCodecOptions
	GCP     GCPCodecOptions
	HTTPOut OutboundHTTPCodecOptions
	MQTT    MQTTCodecOptions
	Serial  SerialCodecOptions
	UDP     UDPCodecOptions
	Watson  WatsonCodecOptions
}

// ConnectionInfo provides information about the connection.
type ConnectionInfo struct {
	IsConnected bool
	// TLS indicates if the connection uses TLS or not.
	TLS bool
	// RemoteAddr is the address of the remote peer.
	RemoteAddr string
	// PeerCertificates is the certificate chain presented by the peer.
	PeerCertificates []*x509.Certificate
}

// IsEOF returns true when err means "end of file".
func IsEOF(err error) bool {
	ret := (errors.Cause(err) == io.EOF)

	// On Windows, when COM port disappears, the error returned is not an EOF,
	// so here, for lack of anything better, we just compare a substring.
	// TODO(dfrank): try to fix the root cause of it
	if runtime.GOOS == "windows" {
		if err != nil {
			ret = ret || strings.Contains(err.Error(), "The I/O operation has been aborted")
		}
	}
	return ret
}
