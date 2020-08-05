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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	glog "k8s.io/klog/v2"
)

// Note: As of today (2018-04-16), there is no official Go SDK for Azure IoT Devices API,
// hence this dirty hack.

const (
	azureAPVersion   = "2018-01-16"
	AzureDMURLScheme = "azdm"
)

type AzureDMCodecOptions struct {
	ConnectionString string
}

type azureDMCodec struct {
	url           *url.URL
	opts          *AzureDMCodecOptions
	resp          chan *reqInfo
	closeNotifier chan struct{}
	closeOnce     sync.Once
	client        *http.Client
}

type reqInfo struct {
	ID   int64
	resp *http.Response
}

func NewAzureDMCodec(connectURL string, opts *AzureDMCodecOptions) (Codec, error) {
	url, err := url.Parse(connectURL)
	if err != nil || url.Scheme != AzureDMURLScheme || url.Host == "" || url.Path == "" {
		return nil, errors.Errorf("invalid URL %q", url)
	}
	r := &azureDMCodec{
		url:           url,
		opts:          opts,
		closeNotifier: make(chan struct{}),
		resp:          make(chan *reqInfo),
		client:        &http.Client{Transport: &http.Transport{}},
	}
	if _, err := r.getSASToken(1); err != nil {
		return nil, errors.Annotatef(err, "unable to create SAS token")
	}
	return r, nil
}

func (c *azureDMCodec) String() string {
	return fmt.Sprintf("[azureDMCodec to %q]", c.url)
}

func (c *azureDMCodec) getSASToken(ttl time.Duration) (string, error) {
	var keyName string
	var key []byte
	// Note: URL takes precedence over connection string.
	if c.url.User != nil && c.url.User.Username() != "" {
		if keyEncEsc, ok := c.url.User.Password(); ok && keyEncEsc != "" {
			keyEnc, err := url.QueryUnescape(keyEncEsc)
			if err != nil {
				return "", errors.Annotatef(err, "invalid key escaping")
			}
			key, err = base64.StdEncoding.DecodeString(keyEnc)
			if err != nil {
				return "", errors.Annotatef(err, "invalid key encoding")
			}
			keyName = c.url.User.Username()
		}
	}
	if keyName == "" && c.opts.ConnectionString != "" {
		parts := strings.Split(c.opts.ConnectionString, ";")
		for _, p := range parts {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 {
				break
			}
			switch kv[0] {
			case "HostName":
				if kv[1] != c.url.Host {

					return "", errors.Errorf("connection string host does not match URL: %q vs %q", kv[1], c.url.Host)
				}
			case "SharedAccessKeyName":
				keyName = kv[1]
			case "SharedAccessKey":
				var err error
				key, err = base64.StdEncoding.DecodeString(kv[1])
				if err != nil {
					return "", errors.Annotatef(err, "invalid key encoding")
				}
			}
		}
		if keyName == "" || key == nil {
			return "", errors.Errorf("invalid connection string format")
		}
	} else if keyName == "" {
		return "", errors.Errorf("neither user:password nor connection string are specified")
	}

	sr := url.QueryEscape(c.url.Host)
	se := time.Now().Add(ttl).Unix()
	pt := fmt.Sprintf("%s\n%d", sr, se)
	hm := hmac.New(sha256.New, key)
	hm.Write([]byte(pt))
	sig := hm.Sum(nil)

	tok := fmt.Sprintf("SharedAccessSignature sr=%s&skn=%s&se=%d&sig=%s",
		sr, url.QueryEscape(keyName), se,
		url.QueryEscape(base64.StdEncoding.EncodeToString(sig)))

	glog.V(2).Infof("tok %s", tok)

	return tok, nil
}

type azureDMReq struct {
	MethodName       string           `json:"methodName"`
	TimeoutInSeconds int64            `json:"timeoutInSeconds"`
	Payload          *json.RawMessage `json:"payload"`
}

type azureDMResp struct {
	Status  int64           `json:"status"`
	Payload json.RawMessage `json:"payload"`
}

func (c *azureDMCodec) Send(ctx context.Context, f *frame.Frame) error {
	select {
	case <-c.closeNotifier:
		return errors.Trace(io.EOF)
	case <-c.resp:
		return errors.Trace(io.EOF)
	default:
	}
	if f.Method == "" {
		return errors.NotImplementedf("responses are not supported")
	}
	if d, ok := ctx.Deadline(); ok {
		c.client.Timeout = time.Until(d)
	}
	if f.Deadline > 0 {
		c.client.Timeout = time.Duration(f.Timeout) * time.Second
	} else if f.Timeout > 0 {
		c.client.Timeout = time.Until(time.Unix(f.Deadline, 0))
	}
	httpsURL := url.URL{
		Scheme:   "https",
		Host:     c.url.Host,
		Path:     fmt.Sprintf("twins%s/methods", c.url.Path),
		RawQuery: "api-version=" + azureAPVersion,
	}
	rq := azureDMReq{
		MethodName:       f.Method,
		TimeoutInSeconds: int64(c.client.Timeout),
	}
	if f.Params != nil {
		rq.Payload = &f.Params
	}
	body, err := json.Marshal(&rq)
	if err != nil {
		return errors.Trace(err)
	}
	glog.V(2).Infof("%s %s", httpsURL.String(), body)
	req, _ := http.NewRequest("POST", httpsURL.String(), bytes.NewBuffer(body))
	sig, err := c.getSASToken(c.client.Timeout)
	if err != nil {
		return errors.Trace(err)
	}
	req.Header.Add("Authorization", sig)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	resp, err := c.client.Do(req)
	if resp != nil {
		c.resp <- &reqInfo{
			ID:   f.ID,
			resp: resp,
		}
	}
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

type azureErrorResp struct {
	Message string
}

func (c *azureDMCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	select {
	case <-ctx.Done():
	case <-c.closeNotifier:
	case ri := <-c.resp:
		defer ri.resp.Body.Close()
		if ri.resp.StatusCode != 200 {
			var em azureErrorResp
			if err := json.NewDecoder(ri.resp.Body).Decode(&em); err != nil {
				return nil, errors.Trace(io.EOF)
			}
			return &frame.Frame{
				Version: 2,
				ID:      ri.ID,
				Error: &frame.Error{
					Code:    ri.resp.StatusCode,
					Message: em.Message,
				},
			}, nil
		}
		var aresp azureDMResp
		if err := json.NewDecoder(ri.resp.Body).Decode(&aresp); err != nil {
			return &frame.Frame{
				Version: 2,
				ID:      ri.ID,
				Error: &frame.Error{
					Code:    400,
					Message: "bad response format",
				},
			}, nil
		}
		return &frame.Frame{
			Version: 2,
			ID:      ri.ID,
			Result:  aresp.Payload,
		}, nil
	}
	return nil, errors.Trace(io.EOF)
}

func (c *azureDMCodec) Close() {
	c.closeOnce.Do(func() { close(c.closeNotifier) })
}

func (c *azureDMCodec) CloseNotify() <-chan struct{} {
	return c.closeNotifier
}

func (c *azureDMCodec) MaxNumFrames() int {
	return 1 // We only ever send one frame.
}

func (c *azureDMCodec) Info() ConnectionInfo {
	return ConnectionInfo{RemoteAddr: c.url.String()}
}

func (c *azureDMCodec) SetOptions(opts *Options) error {
	return errors.NotImplementedf("SetOptions")
}
