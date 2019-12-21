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
package common

import (
	"io"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

const (
	// https://tools.ietf.org/html/rfc1055
	slipFrameDelimiter       = 0xC0
	slipEscape               = 0xDB
	slipEscapeFrameDelimiter = 0xDC
	slipEscapeEscape         = 0xDD
)

type SLIPReaderWriter struct {
	rw io.ReadWriter
}

func NewSLIPReaderWriter(rw io.ReadWriter) *SLIPReaderWriter {
	return &SLIPReaderWriter{rw: rw}
}

func (srw *SLIPReaderWriter) Read(buf []byte) (int, error) {
	n := 0
	start := true
	esc := false
	for {
		b := []byte{0}
		bn, err := srw.rw.Read(b)
		if err != nil || bn != 1 {
			return n, errors.Annotatef(err, "error reading")
		}
		if start {
			if b[0] != slipFrameDelimiter {
				return 0, errors.Errorf("invalid SLIP starting byte: 0x%02x", b[0])
			}
			start = false
			continue
		}
		if !esc {
			switch b[0] {
			case slipFrameDelimiter:
				glog.V(4).Infof("<= (%d) %s", n, LimitStr(buf[:n], 32))
				return n, nil
			case slipEscape:
				esc = true
			default:
				if n >= len(buf) {
					return n, errors.Errorf("frame buffer overflow (%d)", len(buf))
				}
				buf[n] = b[0]
				n += 1
			}
		} else {
			if n >= len(buf) {
				return n, errors.Errorf("frame buffer overflow (%d)", len(buf))
			}
			switch b[0] {
			case slipEscapeFrameDelimiter:
				buf[n] = slipFrameDelimiter
			case slipEscapeEscape:
				buf[n] = slipEscape
			default:
				return n, errors.Errorf("invalid SLIP escape sequence: %d", b[0])
			}
			n += 1
			esc = false
		}
	}
}

func (srw *SLIPReaderWriter) Write(data []byte) (int, error) {
	frame := []byte{slipFrameDelimiter}
	for _, b := range data {
		switch b {
		case slipFrameDelimiter:
			frame = append(frame, slipEscape)
			frame = append(frame, slipEscapeFrameDelimiter)
		case slipEscape:
			frame = append(frame, slipEscape)
			frame = append(frame, slipEscapeEscape)
		default:
			frame = append(frame, b)
		}
	}
	frame = append(frame, slipFrameDelimiter)
	glog.V(4).Infof("=> (%d) %s", len(data), LimitStr(data, 32))
	return srw.rw.Write(frame)
}
