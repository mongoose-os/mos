package mqtt

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func mqttexpect(t *testing.T, c chan MQTT.Message, topic, payload string) {
	select {
	case m := <-c:
		// log.Println("exp", topic, "->", string(m.Payload()))
		if m.Topic() != topic || string(m.Payload()) != payload {
			t.Fatal(m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}
}

func TestNoAuthConnectDisconnect(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	b := NewBroker(nil)
	go b.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	c.Disconnect(250)
}

func TestUserPasswordAuth(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	b := NewBroker(&Hooks{
		Auth: func(c *Client) error {
			if c.Username != "test" && c.Password != "t3$t" {
				return errors.New("bad credentials")
			}
			return nil
		},
	})
	go b.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() == nil {
		t.Fatal()
	}
	opts.SetUsername("test")
	opts.SetPassword("t3$t")
	c = MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	c.Disconnect(250)
}

func TestSimplePubSub(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	b := NewBroker(nil)
	go b.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	msgs := make(chan MQTT.Message, 10)
	opts.SetDefaultPublishHandler(func(c MQTT.Client, m MQTT.Message) {
		msgs <- m
	})
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	defer c.Disconnect(250)

	// Single topic
	if token := c.Subscribe("test/foo", 0, nil); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}

	// Multiple topics
	if token := c.SubscribeMultiple(map[string]byte{"test/bar": 0, "test/baz": 0}, nil); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}

	c.Publish("test/foo", 0, false, "msg1").Wait()
	mqttexpect(t, msgs, "test/foo", "msg1")

	c.Publish("test/baz", 0, false, "msg2").Wait()
	mqttexpect(t, msgs, "test/baz", "msg2")

	if !c.Unsubscribe("test/baz").WaitTimeout(time.Second) {
		t.Fatal()
	}
	if !c.Publish("test/baz", 0, false, "msg3").Wait() {
		t.Fatal()
	}
	select {
	case m := <-msgs:
		t.Fatal(m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestQoS1(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	broker := NewBroker(nil)
	go broker.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	defer c.Disconnect(250)
	msgc := make(chan MQTT.Message, 1)
	if token := c.Subscribe("test/foo", 1, func(c MQTT.Client, m MQTT.Message) {
		msgc <- m
	}); token.WaitTimeout(time.Millisecond*200) && token.Error() != nil {
		t.Fatal(token.Error())
	}
	c.Publish("test/foo", 1, false, "hello world").Wait()
	select {
	case m := <-msgc:
		if m.Topic() != "test/foo" || string(m.Payload()) != "hello world" || m.Qos() != 1 {
			t.Fatal(m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}
}

func TestWill(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	broker := NewBroker(nil)
	go broker.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}

	msgc := make(chan MQTT.Message, 1)
	if token := c.Subscribe("test/bye", 1, func(c MQTT.Client, m MQTT.Message) {
		msgc <- m
	}); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}

	go func() {
		opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
		opts.SetWill("test/bye", "goodbye!", 0, false)
		c := MQTT.NewClient(opts)
		if token := c.Connect(); token.Wait() && token.Error() != nil {
			t.Fatal(token.Error())
		}
		c.Disconnect(250)
	}()
	mqttexpect(t, msgc, "test/bye", "goodbye!")
}

func TestWebsocket(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	h := &WebSocketHandler{}
	http.Handle("/ws", h)
	go http.Serve(ln, nil)
	broker := NewBroker(nil)
	go broker.Run(h)

	opts := MQTT.NewClientOptions().AddBroker("ws://" + ln.Addr().String() + "/ws")
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	defer c.Disconnect(250)
}

func TestHooks(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	broker := NewBroker(&Hooks{
		Auth: func(c *Client) error {
			c.TopicPrefix = c.Username + "/"
			return nil
		},
		Publish: func(c *Client, topic string, payload []byte) error {
			if topic == "bar" {
				upper := bytes.ToUpper(payload)
				for i := range upper {
					payload[i] = upper[i]
				}
			}
			// bytes.ToUpper(payload)
			return nil
		},
		Subscribe: func(c *Client, topic string) error {
			if topic == "baz" {
				return fmt.Errorf("nope!")
			}
			return nil
		},
	})
	go broker.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	opts.SetUsername("coolio")
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	defer c.Disconnect(250)

	opts2 := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	opts2.SetUsername("bobby")
	c2 := MQTT.NewClient(opts2)
	if token := c2.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	defer c2.Disconnect(250)

	msgc := make(chan MQTT.Message, 2)
	if token := c.Subscribe("foo", 0, func(c MQTT.Client, m MQTT.Message) {
		msgc <- m
	}); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	if token := c.Subscribe("bar", 0, func(c MQTT.Client, m MQTT.Message) {
		msgc <- m
	}); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}

	c2.Publish("foo", 0, false, "must not be received by c").Wait()
	c.Publish("foo", 1, false, "hello world").Wait()
	mqttexpect(t, msgc, "foo", "hello world")

	c.Publish("bar", 0, false, "hello world").Wait()
	mqttexpect(t, msgc, "bar", "HELLO WORLD")
}

func BenchmarkPublishSubscribe(b *testing.B) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()
	broker := NewBroker(nil)
	go broker.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		b.Fatal(token.Error())
	}
	defer c.Disconnect(250)
	if token := c.Subscribe("test/foo", 0, nil); token.Wait() && token.Error() != nil {
		b.Fatal(token.Error())
	}
	for i := 0; i < b.N; i++ {
		c.Publish("test/foo", 0, false, "hello world").Wait()
	}
}
