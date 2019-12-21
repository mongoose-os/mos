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
	"flag"
	"time"

	"github.com/juju/errors"
)

type Client struct {
	Port      string
	Reconnect bool
	Timeout   time.Duration
}

func (c *Client) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Port, "port", "", "Serial port to use")
	fs.BoolVar(&c.Reconnect, "reconnect", false, "Enable serial port reconnection")
	fs.DurationVar(&c.Timeout, "timeout", 10*time.Second,
		"Timeout for the device connection")
}

func (c *Client) PostProcessFlags(fs *flag.FlagSet) error {
	if c.Port == "" {
		return errors.New("-port is required")
	}

	return nil
}

func UsageSummary() string {
	return "-port <port-name>"
}

// RunWithTimeout takes a parent context and a function, and calls the function
// with the newly created context with timeout (see the "timeout" flag)
func (c *Client) RunWithTimeout(ctx context.Context, f func(context.Context) error) error {
	cctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	return errors.Trace(f(cctx))
}
