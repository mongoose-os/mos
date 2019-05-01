package codec

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"cesanta.com/common/go/mgrpc/frame"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

type outboundHttpCodec struct {
	sync.Mutex
	closeNotifier chan struct{}
	closeOnce     sync.Once
	url           string
	queue         []*frame.Frame
	cond          *sync.Cond
	client        *http.Client
}

// OutboundHTTP sends outbound frames in HTTP POST requests and
// returns replies with Recv.
func OutboundHTTP(url string, tlsConfig *tls.Config) Codec {
	r := &outboundHttpCodec{
		closeNotifier: make(chan struct{}),
		url:           url,
		client:        &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}},
	}
	r.cond = sync.NewCond(r)
	return r
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
	glog.V(2).Infof("Sending to %q over HTTP POST: %q", c.url, string(b))
	// TODO(imax): use http.Client to set the timeout.
	resp, err := c.client.Post(c.url, "application/json", bytes.NewReader(b))
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned an error: %v", resp)
	}
	var rfs *frame.Frame
	if err := json.NewDecoder(resp.Body).Decode(&rfs); err != nil {
		// Return it from Recv?
		return errors.Trace(err)
	}
	c.Lock()
	c.queue = append(c.queue, rfs)
	c.Unlock()
	c.cond.Signal()
	return nil
}

func (c *outboundHttpCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	// Check if there's anything left in the queue.
	var r *frame.Frame
	c.Lock()
	if len(c.queue) > 0 {
		r, c.queue = c.queue[0], c.queue[1:]
	}
	c.Unlock()
	if r != nil {
		return r, nil
	}
	// Wait for stuff to arrive.
	ch := make(chan *frame.Frame, 1)
	go func(ctx context.Context) {
		c.Lock()
		defer c.Unlock()
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
