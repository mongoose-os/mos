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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudiot/v1"
	glog "k8s.io/klog/v2"
)

const (
	// gcp://project/region/registry/device?topic=...&sub=...&reqsf=....&respsf=...
	GCPURLScheme = "gcp"

	topicQSParam    = "topic"
	defaultGCPTopic = "rpc"

	subscriptionQSParam    = "sub"
	defaultGCPSubscription = "rpc"

	reqSubfolderQSParam    = "reqsf"
	defaultGCPReqSubfolder = "rpc"

	respSubfolderQSParam    = "respsf"
	defaultGCPRespSubfolder = "rpc"
)

type GCPCodecOptions struct {
	CreateTopic bool
}

type gcpCodec struct {
	opts     *GCPCodecOptions
	name     string
	project  string
	region   string
	registry string
	device   string

	topic        string
	subscription string

	reqSubfolder  string
	respSubfolder string

	reqs sync.Map
	resp chan *frame.Frame

	closeNotifier chan struct{}
	closeOnce     sync.Once
	iot           *cloudiot.Service

	pubSub          *pubsub.Client
	pubSubCtx       context.Context
	pubSubRecvCtx   context.Context
	pubSubCtxCancel context.CancelFunc
	sub             *pubsub.Subscription

	recvCancel context.CancelFunc
	recvDone   chan struct{}
}

func qsvalueOrDefault(vv url.Values, key, dfl string) string {
	sf, ok := vv[key]
	if ok {
		return sf[0]
	} else {
		return dfl
	}
}

func NewGCPCodec(connectURL string, opts *GCPCodecOptions) (Codec, error) {
	url, err := url.Parse(connectURL)
	parts := strings.Split(url.Path, "/")
	if err != nil || url.Scheme != GCPURLScheme || url.Host == "" || url.Path == "" || len(parts) != 4 {
		return nil, errors.Errorf("invalid URL %q, should be gcp://project/region/registry/device", url)
	}
	vv := url.Query()
	httpClient, err := google.DefaultClient(context.Background(), cloudiot.CloudPlatformScope)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to create GCP HTTP client")
	}
	iot, err := cloudiot.New(httpClient)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to create GCP client")
	}
	project := url.Host
	glog.V(1).Infof("Creating gPubSub client for %s", project)
	pctx, pctxCancel := context.WithCancel(context.Background())
	pubSub, err := pubsub.NewClient(pctx, project)
	if err != nil {
		return nil, errors.Trace(err)
	}
	r := &gcpCodec{
		project:  project,
		region:   parts[1],
		registry: parts[2],
		device:   parts[3],
		opts:     opts,

		topic:        qsvalueOrDefault(vv, topicQSParam, defaultGCPTopic),
		subscription: qsvalueOrDefault(vv, subscriptionQSParam, defaultGCPSubscription),

		reqSubfolder:  qsvalueOrDefault(vv, reqSubfolderQSParam, defaultGCPReqSubfolder),
		respSubfolder: qsvalueOrDefault(vv, respSubfolderQSParam, defaultGCPRespSubfolder),

		closeNotifier: make(chan struct{}),
		resp:          make(chan *frame.Frame),
		iot:           iot,

		pubSub:          pubSub,
		pubSubCtx:       pctx,
		pubSubCtxCancel: pctxCancel,
	}

	glog.Infof("Created %s", r)

	if err = r.subscribe(); err != nil {
		r.Close()
		return nil, errors.Annotatef(err, "failed to subscribe to response topic")
	}

	return r, nil
}

func (c *gcpCodec) createEventNotificationConfig() error {
	// Verify that subfolder -> topic forwarding rule exists.
	regName := fmt.Sprintf("projects/%s/locations/%s/registries/%s",
		c.project, c.region, c.registry)
	reg, err := c.iot.Projects.Locations.Registries.Get(regName).Do()
	if err != nil {
		return errors.Trace(err)
	}
	glog.V(4).Infof("%#v", reg)
	topicName := fmt.Sprintf("projects/%s/topics/%s", c.project, c.topic)
	dflt := ""
	var newConfigs []*cloudiot.EventNotificationConfig
	for _, enc := range reg.EventNotificationConfigs {
		glog.V(4).Infof("%s %s", enc.SubfolderMatches, enc.PubsubTopicName)
		if enc.SubfolderMatches == c.respSubfolder {
			if enc.PubsubTopicName != topicName {
				return errors.Errorf("event notification config for subfolder %q exists but points to %q instead of %q",
					c.respSubfolder, enc.PubsubTopicName, topicName,
				)
			}
			return nil
		} else if enc.SubfolderMatches != "" {
			newConfigs = append(newConfigs, enc)
		} else {
			dflt = enc.PubsubTopicName
		}
	}
	// Insert our entry at the end. But default must be the last.
	newConfigs = append(newConfigs, &cloudiot.EventNotificationConfig{
		SubfolderMatches: c.respSubfolder,
		PubsubTopicName:  topicName,
	})
	newConfigs = append(newConfigs, &cloudiot.EventNotificationConfig{
		PubsubTopicName: dflt,
	})
	glog.Infof("Adding EventNotification config for %s -> %s", c.respSubfolder, topicName)
	_, err = c.iot.Projects.Locations.Registries.Patch(regName, &cloudiot.DeviceRegistry{
		EventNotificationConfigs: newConfigs,
	}).UpdateMask("event_notification_configs").Do()
	if err == nil {
		// Give the change some time to propagate.
		time.Sleep(8 * time.Second)
	}
	return errors.Annotatef(err, "add EventNotificationConfig")
}

func (c *gcpCodec) subscribe() error {
	sub := c.pubSub.Subscription(c.subscription)
	ok, err := sub.Exists(c.pubSubCtx)
	if err != nil {
		return errors.Trace(err)
	}
	if !ok {
		glog.Warningf("Subscription %q does not exist, creating...", c.subscription)
		topic := c.pubSub.Topic(c.topic)
		ok, err = topic.Exists(c.pubSubCtx)
		if err != nil {
			return errors.Trace(err)
		}
		if !ok {
			if c.opts.CreateTopic {
				glog.Warningf("Topic %q does not exist, creating...", c.topic)
				topic, err = c.pubSub.CreateTopic(c.pubSubCtx, c.topic)
				if err != nil {
					return errors.Annotatef(err, "topic %q does not exist and could not be created", c.topic)
				}
			} else {
				return errors.Errorf("topic %q does not exist and --gcp-rpc-create-topic is not set", c.topic)
			}
		} else {
			glog.V(4).Infof("Topic %q exists...", c.topic)
		}
		if err := c.createEventNotificationConfig(); err != nil {
			return errors.Annotatef(err, "failed to create registry event notifications config")
		}
		sub, err = c.pubSub.CreateSubscription(
			c.pubSubCtx, c.subscription,
			pubsub.SubscriptionConfig{
				Topic:       topic,
				AckDeadline: 10 * time.Second,
				// RPC responses are transient, use shortest possible retention.
				RetentionDuration: 10 * time.Minute,
			},
		)
		if err != nil {
			return errors.Annotatef(err, "subscription %q does not exist and could not be created", c.subscription)
		}
	}
	c.recvDone = make(chan struct{})
	pubSubRecvCtx, rc := context.WithCancel(c.pubSubCtx)
	c.recvCancel = rc
	go func() {
		glog.V(4).Infof("Receiving messages from %s", sub)
		err = sub.Receive(pubSubRecvCtx, func(ctx context.Context, m *pubsub.Message) {
			// Make sure the message is addressed to us - all the attributes must match.
			// These attributes are added by the MQTT bridge.
			// https://cloud.google.com/iot/docs/how-tos/mqtt-bridge#publishing_telemetry_events
			aa := m.Attributes
			glog.V(4).Infof("%s %s %d", m.ID, m.Attributes, len(m.Data))
			if aa["projectId"] != c.project ||
				aa["deviceRegistryLocation"] != c.region ||
				aa["deviceRegistryId"] != c.registry ||
				aa["deviceId"] != c.device ||
				aa["subFolder"] != c.respSubfolder {
				glog.V(4).Infof("Unrelated message")
				m.Nack()
				return
			}
			f := &frame.Frame{}
			if err := json.Unmarshal(m.Data, f); err != nil {
				glog.Errorf("Invalid json (%s): %s", err, m.Data)
				m.Nack()
				return
			}
			glog.V(4).Infof("%s", string(m.Data))
			if _, ok := c.reqs.Load(f.ID); !ok {
				glog.V(4).Infof("Unrelated request (id %d)", f.ID)
				// Clean up old unrelated responses.
				// Caller is most likely long gone, they just clog up the pipes and cause churn.
				if m.PublishTime.Before(time.Now().Add(-1 * time.Minute)) {
					m.Ack()
				} else {
					// Somebody else is waiting for this response, throw back on the pile.
					m.Ack()
				}
				return
			}
			m.Ack()
			c.reqs.Delete(f.ID)
			c.resp <- f
		})
		glog.V(4).Infof("Subscriber out YYY %s", err)
		if err != nil && err != context.Canceled {
			c.Close()
		}
		glog.V(4).Infof("Subscriber out")
		close(c.recvDone)
	}()
	return nil
}

func (c *gcpCodec) GetURL() string {
	u := url.URL{
		Scheme: GCPURLScheme,
		Host:   c.project,
		Path:   fmt.Sprintf("/%s/%s/%s", c.region, c.registry, c.device),
	}
	q := u.Query()
	q.Set(topicQSParam, c.topic)
	q.Set(reqSubfolderQSParam, c.reqSubfolder)
	q.Set(respSubfolderQSParam, c.respSubfolder)
	q.Set(subscriptionQSParam, c.subscription)
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *gcpCodec) String() string {
	return fmt.Sprintf("[gcpCodec %s]", c.GetURL())
}

func (c *gcpCodec) Send(ctx context.Context, f *frame.Frame) error {
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
	f.Src = c.respSubfolder
	fj, err := json.Marshal(f)
	if err != nil {
		return errors.Trace(err)
	}
	name := fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s",
		c.project, c.region, c.registry, c.device)
	glog.V(2).Infof("%s -> %s", fj, name)
	req := cloudiot.SendCommandToDeviceRequest{
		BinaryData: base64.StdEncoding.EncodeToString(fj),
		Subfolder:  c.reqSubfolder,
	}

	c.reqs.Store(f.ID, true)
	_, err = c.iot.Projects.Locations.Registries.Devices.SendCommandToDevice(name, &req).Do()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *gcpCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	select {
	case <-ctx.Done():
	case <-c.closeNotifier:
	case f, ok := <-c.resp:
		if ok {
			return f, nil
		}
	}
	return nil, errors.Trace(io.EOF)
}

func (c *gcpCodec) Close() {
	if c.recvDone != nil {
		c.recvCancel()
		<-c.recvDone
	}
	c.pubSubCtxCancel()
	c.closeOnce.Do(func() { close(c.closeNotifier) })
	c.pubSub.Close()
}

func (c *gcpCodec) CloseNotify() <-chan struct{} {
	return c.closeNotifier
}

func (c *gcpCodec) MaxNumFrames() int {
	return -1
}

func (c *gcpCodec) Info() ConnectionInfo {
	return ConnectionInfo{RemoteAddr: c.name}
}

func (c *gcpCodec) SetOptions(opts *Options) error {
	return errors.NotImplementedf("SetOptions")
}
