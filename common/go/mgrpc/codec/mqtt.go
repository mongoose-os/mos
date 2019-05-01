package codec

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"cesanta.com/common/go/mgrpc/frame"

	"github.com/cesanta/errors"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
)

type MQTTCodecOptions struct {
	// Note: user:password in the connect URL, if set, will take precedence over these.
	User     string
	Password string
	ClientID string
	PubTopic string
	SubTopic string
	Src      string
}

type mqttCodec struct {
	src         string
	dst         string
	closeNotify chan struct{}
	ready       chan struct{}
	rchan       chan frame.Frame
	cli         mqtt.Client
	closeOnce   sync.Once
	isTLS       bool
	pubTopic    string
}

func MQTTClientOptsFromURL(us, clientID, user, pass string) (*mqtt.ClientOptions, string, error) {
	u, err := url.Parse(us)
	if err != nil {
		return nil, "", errors.Trace(err)
	}

	if clientID == "" {
		clientID = fmt.Sprintf("mos-%v", time.Now().Unix())
	}

	topic := u.Path[1:]

	u.Path = ""
	if u.Scheme == "mqtts" {
		u.Scheme = "tcps"
		if u.Port() == "" {
			u.Host = fmt.Sprintf("%s:%d", u.Host, 8883)
		}
	} else {
		u.Scheme = "tcp"
		if u.Port() == "" {
			u.Host = fmt.Sprintf("%s:%d", u.Host, 1883)
		}
	}
	broker := u.String()
	glog.V(1).Infof("Connecting %s to %s", clientID, broker)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if u.User != nil {
		user = u.User.Username()
		passwd, isset := u.User.Password()
		if isset {
			pass = passwd
		}
	}
	opts.SetUsername(user)
	opts.SetPassword(pass)

	return opts, topic, nil
}

func MQTT(dst string, tlsConfig *tls.Config, co *MQTTCodecOptions) (Codec, error) {
	opts, topic, err := MQTTClientOptsFromURL(dst, co.ClientID, co.User, co.Password)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if tlsConfig != nil {
		opts.SetTLSConfig(tlsConfig)
	}

	u, _ := url.Parse(dst)

	c := &mqttCodec{
		dst:         topic,
		closeNotify: make(chan struct{}),
		ready:       make(chan struct{}),
		rchan:       make(chan frame.Frame),
		src:         co.Src,
		pubTopic:    co.PubTopic,
		isTLS:       (u.Scheme == "mqtts"),
	}
	if c.src == "" {
		c.src = opts.ClientID
	}

	opts.SetConnectionLostHandler(c.onConnectionLost)

	c.cli = mqtt.NewClient(opts)
	token := c.cli.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, errors.Annotatef(err, "MQTT connect error")
	}

	subTopic := co.SubTopic
	if subTopic == "" {
		subTopic = fmt.Sprintf("%s/rpc", c.src)
	}
	glog.V(1).Infof("Subscribing to [%s]", subTopic)
	token = c.cli.Subscribe(subTopic, 1 /* qos */, c.onMessage)
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, errors.Annotatef(err, "MQTT subscribe error")
	}

	return c, nil
}

func (c *mqttCodec) onMessage(cli mqtt.Client, msg mqtt.Message) {
	glog.V(4).Infof("Got MQTT message, topic [%s], message [%s]", msg.Topic(), msg.Payload())
	f := &frame.Frame{}
	if err := json.Unmarshal(msg.Payload(), &f); err != nil {
		glog.Errorf("Invalid json (%s): %+v", err, msg.Payload())
		return
	}
	c.rchan <- *f
}

func (c *mqttCodec) onConnectionLost(cli mqtt.Client, err error) {
	glog.Errorf("Lost conection to MQTT broker: %s", err)
	c.Close()
}

func (c *mqttCodec) Close() {
	c.closeOnce.Do(func() {
		glog.V(1).Infof("Closing %s", c)
		close(c.closeNotify)
		c.cli.Disconnect(0)
	})
}

func (c *mqttCodec) CloseNotify() <-chan struct{} {
	return c.closeNotify
}

func (c *mqttCodec) String() string {
	return fmt.Sprintf("[mqttCodec to %s]", c.dst)
}

func (c *mqttCodec) Info() ConnectionInfo {
	return ConnectionInfo{
		IsConnected: c.cli.IsConnected(),
		TLS:         c.isTLS,
		RemoteAddr:  c.dst,
	}
}

func (c *mqttCodec) MaxNumFrames() int {
	return -1
}

func (c *mqttCodec) Recv(ctx context.Context) (*frame.Frame, error) {
	select {
	case f := <-c.rchan:
		return &f, nil
	case <-c.closeNotify:
		return nil, errors.Trace(io.EOF)
	}
}

func (c *mqttCodec) Send(ctx context.Context, f *frame.Frame) error {
	f.Src = c.src
	msg, err := json.Marshal(f)
	if err != nil {
		return errors.Trace(err)
	}
	if f.Dst == "" {
		f.Dst = c.dst
	}
	topic := c.pubTopic
	if topic == "" {
		topic = fmt.Sprintf("%s/rpc", f.Dst)
	}
	glog.V(4).Infof("Sending [%s] to [%s]", msg, topic)
	token := c.cli.Publish(topic, 1 /* qos */, false /* retained */, msg)
	token.Wait()
	if err := token.Error(); err != nil {
		return errors.Annotatef(err, "MQTT publish error")
	}
	return nil
}

func (c *mqttCodec) SetOptions(opts *Options) error {
	return errors.NotImplementedf("SetOptions")
}
