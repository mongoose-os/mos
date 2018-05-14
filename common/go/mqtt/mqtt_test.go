package mqtt

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

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
		Auth: func(id, username, password string) error {
			if username != "test" && password != "t3$t" {
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
	select {
	case m := <-msgs:
		if m.Topic() != "test/foo" || string(m.Payload()) != "msg1" {
			t.Fatal(m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

	c.Publish("test/baz", 0, false, "msg2").Wait()
	select {
	case m := <-msgs:
		if m.Topic() != "test/baz" || string(m.Payload()) != "msg2" {
			t.Fatal(m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

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
	}); token.Wait() && token.Error() != nil {
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

	select {
	case m := <-msgc:
		if m.Topic() != "test/bye" || string(m.Payload()) != "goodbye!" {
			t.Fatal(m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}
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
		Publish: func(id, user, passwd, topic string, payload []byte) (string, []byte, error) {
			if topic == "foo" {
				return "test/foo", bytes.ToUpper(payload), nil
			}
			return topic, payload, nil
		},
		Subscribe: func(id, user, passwd, topic string) (string, error) {
			if topic == "bar" {
				return "test/bar", nil
			}
			return topic, nil
		},
	})
	go broker.Run(ln)

	opts := MQTT.NewClientOptions().AddBroker("tcp://" + ln.Addr().String())
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	defer c.Disconnect(250)

	msgc := make(chan MQTT.Message, 2)
	if token := c.Subscribe("test/foo", 0, func(c MQTT.Client, m MQTT.Message) {
		msgc <- m
	}); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}
	if token := c.Subscribe("bar", 0, func(c MQTT.Client, m MQTT.Message) {
		msgc <- m
	}); token.Wait() && token.Error() != nil {
		t.Fatal(token.Error())
	}

	c.Publish("foo", 0, false, "hello world").Wait()
	select {
	case m := <-msgc:
		if m.Topic() != "test/foo" || string(m.Payload()) != "HELLO WORLD" {
			t.Fatal(m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

	c.Publish("test/bar", 0, false, "hello world").Wait()
	select {
	case m := <-msgc:
		if m.Topic() != "bar" || string(m.Payload()) != "hello world" {
			t.Fatal(m)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}
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
