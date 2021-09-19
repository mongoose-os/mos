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
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	glog "k8s.io/klog/v2"
)

type OutboundHTTPCodecOptions struct {
	GetCredsCallback func() (username, passwd string, err error)
}

type outboundHttpCodec struct {
	mu            sync.Mutex
	tlsConfig     *tls.Config
	opts          OutboundHTTPCodecOptions
	closeNotifier chan struct{}
	closeOnce     sync.Once
	url           string
	queue         []*frame.Frame
	cond          *sync.Cond
	client        *http.Client
}

// OutboundHTTP sends outbound frames in HTTP POST requests and
// returns replies with Recv.
func OutboundHTTP(url string, tlsConfig *tls.Config, opts OutboundHTTPCodecOptions) Codec {
	c := &outboundHttpCodec{
		tlsConfig:     tlsConfig,
		opts:          opts,
		closeNotifier: make(chan struct{}),
		url:           url,
	}
	c.cond = sync.NewCond(&c.mu)
	c.createClient()
	return c
}

func (c *outboundHttpCodec) createClient() {
	c.client = &http.Client{Transport: &http.Transport{TLSClientConfig: c.tlsConfig}}
}

func (c *outboundHttpCodec) String() string {
	return fmt.Sprintf("[outboundHttpCodec to %q]", c.url)
}

func (c *outboundHttpCodec) Send(ctx context.Context, f *frame.Frame) error {
	select {
	case <-c.closeNotifier:
		return errors.Trace(io.EOF)
	default:
	}
	b, err := frame.MarshalJSON(f)
	if err != nil {
		return errors.Trace(err)
	}
	return c.sendHTTPRequest(ctx, "", b)
}

func (c *outboundHttpCodec) sendHTTPRequest(ctx context.Context, authHeader string, b []byte) error {
	glog.V(2).Infof("Sending to %q over HTTP POST: %q", c.url, string(b))
	if d, ok := ctx.Deadline(); ok {
		c.client.Timeout = time.Until(d)
	}
	req, err := http.NewRequest("POST", c.url, bytes.NewReader(b))
	if err != nil {
		return errors.Annotatef(err, "failed to create request")
	}
	req.Header.Add("Content-Type", "application/json")
	if authHeader != "" {
		glog.V(2).Infof("Authorization: %s", authHeader)
		req.Header.Add("Authorization", authHeader)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		if authHeader != "" {
			return errors.New("authentication failed")
		}
		ah := resp.Header.Get("WWW-Authenticate")
		if len(ah) == 0 {
			return errors.New("no www-authentiocate header")
		}
		authMethod := regexp.MustCompile(`^(\S+)`).FindString(ah)
		if strings.ToLower(authMethod) != "digest" {
			return fmt.Errorf("invalid auth method %q", authMethod)
		}
		pp := make(map[string]string)
		for _, m := range regexp.MustCompile(`(\w+)="([^"]*?)"|(\w+)=([^\s"]+)`).FindAllStringSubmatch(ah, -1) {
			pp[strings.ToLower(m[1])] = m[2]
			pp[strings.ToLower(m[3])] = m[4]
		}
		glog.Infof("%+v", pp)
		if c.opts.GetCredsCallback == nil {
			return errors.New("authorization required but no credentials callback provided")
		}
		username, passwd, err := c.opts.GetCredsCallback()
		if err != nil {
			return errors.Annotatef(err, "error getting credentials")
		}
		authAlgo := pp["algorithm"]
		if authAlgo == "" {
			authAlgo = "MD5"
		}
		nonce := pp["nonce"]
		cnonce, authResp, err := MkDigestResp("POST", req.URL.Path, username, pp["realm"], passwd, pp["algorithm"], nonce, "00000001", pp["qop"])
		authHeader = fmt.Sprintf(
			`%s username="%s", realm="%s", uri="%s", algorithm=%s, nonce="%s", nc=%08x, cnonce="%d", qop=%s, response="%s"`,
			authMethod, username, pp["realm"], req.URL.Path, authAlgo, nonce, 1, cnonce, pp["qop"], authResp,
		)
		if pp["opaque"] != "" {
			authHeader = fmt.Sprintf(`%s, opaque="%s"`, authHeader, pp["opaque"])
		}
		c.createClient()
		return c.sendHTTPRequest(ctx, authHeader, b)
	default:
		return fmt.Errorf("server returned an error: %v", resp)
	}
	var rfs *frame.Frame
	if err := json.NewDecoder(resp.Body).Decode(&rfs); err != nil {
		// Return it from Recv?
		return errors.Trace(err)
	}
	c.mu.Lock()
	c.queue = append(c.queue, rfs)
	c.mu.Unlock()
	c.cond.Signal()
	return nil
}

func (c *outboundHttpCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	// Check if there's anything left in the queue.
	var r *frame.Frame
	c.mu.Lock()
	if len(c.queue) > 0 {
		r, c.queue = c.queue[0], c.queue[1:]
	}
	c.mu.Unlock()
	if r != nil {
		return r, nil
	}
	// Wait for stuff to arrive.
	ch := make(chan *frame.Frame, 1)
	go func(ctx context.Context) {
		c.mu.Lock()
		defer c.mu.Unlock()
		for len(c.queue) == 0 {
			select {
			case <-ctx.Done():
				return
			default:
			}
			c.cond.Wait()
		}
		var f *frame.Frame
		f, c.queue = c.queue[0], c.queue[1:]
		ch <- f // chan is buffered so we won't be stuck forever if the reader is gone
	}(ctx)
	select {
	case r = <-ch:
		return r, nil
	case <-c.closeNotifier:
		return nil, errors.Trace(io.EOF)
	}
}

func (c *outboundHttpCodec) Close() {
	c.closeOnce.Do(func() { close(c.closeNotifier) })
}

func (c *outboundHttpCodec) CloseNotify() <-chan struct{} {
	return c.closeNotifier
}

func (c *outboundHttpCodec) MaxNumFrames() int {
	return 1 // We only ever send one frame.
}

func (c *outboundHttpCodec) Info() ConnectionInfo {
	return ConnectionInfo{RemoteAddr: c.url}
}

func (c *outboundHttpCodec) SetOptions(opts *Options) error {
	return errors.NotImplementedf("SetOptions")
}

func MkDigestResp(method, uri, username, realm, passwd, algorithm, nonce, nc, qop string) (int, string, error) {
	var hashFunc func(data []byte) []byte
	switch algorithm {
	case "":
		fallthrough
	case "MD5":
		hashFunc = func(data []byte) []byte {
			s := md5.Sum(data)
			return s[:]
		}
	case "SHA-256":
		hashFunc = func(data []byte) []byte {
			s := sha256.Sum256(data)
			return s[:]
		}
	default:
		return 0, "", fmt.Errorf("unknown digest algorithm %q", algorithm)
	}

	cnonceBig, err := rand.Int(rand.Reader, big.NewInt(0xffffffff))
	if err != nil {
		return 0, "", errors.Annotatef(err, "generating cnonce")
	}
	cnonce := int(cnonceBig.Int64())

	ha1 := hex.EncodeToString(hashFunc([]byte(fmt.Sprintf("%s:%s:%s", username, realm, passwd))))

	ha2 := hex.EncodeToString(hashFunc([]byte(fmt.Sprintf("%s:%s", method, uri))))

	resp := hex.EncodeToString(hashFunc([]byte(fmt.Sprintf("%s:%s:%s:%d:%s:%s", ha1, nonce, nc, cnonce, qop, ha2))))

	return cnonce, resp, nil
}
