package mqtt

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
)

type AuthFunc func(id, username, password string) error
type CloseFunc func(id, username, password string) error
type PublishFunc func(id, username, password, topic string, payload []byte) (string, []byte, error)
type SubscribeFunc func(id, username, password, topic string) (string, error)

type Hooks struct {
	Auth      AuthFunc
	Close     CloseFunc
	Publish   PublishFunc
	Subscribe SubscribeFunc
}

// Broker is a Pub/Sub message forwarder for MQTT protocol.
type Broker interface {
	Run(l net.Listener) error
	Publish(topic string, payload []byte)
}

type broker struct {
	sync.Mutex
	msgID   uint32
	subs    map[string][]*client
	pending map[int]pendingMsg

	auth          AuthFunc
	closeHook     CloseFunc
	publishHook   PublishFunc
	subscribeHook SubscribeFunc
}

type client struct {
	broker   *broker
	conn     net.Conn
	id       string
	username string
	password string
	subAlias map[string]string
	pubAlias map[string]string
}

type pendingMsg struct {
	id int
	c  *client
}

// NewBroker creates a new MQTT broker with a user-defined authorization function
func NewBroker(hooks *Hooks) Broker {
	if hooks == nil {
		hooks = &Hooks{}
	}
	brk := &broker{
		auth:          hooks.Auth,
		closeHook:     hooks.Close,
		publishHook:   hooks.Publish,
		subscribeHook: hooks.Subscribe,
		subs:          map[string][]*client{},
		pending:       map[int]pendingMsg{},
	}
	return brk
}

func (b *broker) Run(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		client := &client{
			broker:   b,
			conn:     conn,
			subAlias: map[string]string{},
			pubAlias: map[string]string{},
		}
		go client.run()
	}
}

func (b *broker) sub(c *client, topics ...string) {
	b.Lock()
	defer b.Unlock()
	for _, topic := range topics {
		b.subs[topic] = append(b.subs[topic], c)
	}
}

func (b *broker) unsub(c *client, topics ...string) {
	b.Lock()
	defer b.Unlock()
	for _, topic := range topics {
		clients := b.subs[topic]
		newClients := make([]*client, 0, len(clients))
		for _, sub := range clients {
			if sub != c {
				newClients = append(newClients, sub)
			}
		}
		b.subs[topic] = newClients
	}
}

func (b *broker) match(topic, wildcard string) bool {
	parts := strings.Split(topic, "/")
	wild := strings.Split(wildcard, "/")
	for i := 0; i < len(parts); i++ {
		if i >= len(wild) {
			return false
		}
		if wild[i] == "#" {
			return true
		}
		if parts[i] != wild[i] && wild[i] != "+" {
			return false
		}
	}
	return len(parts) == len(wild) || (len(parts) == len(wild)-1 && wild[len(wild)-1] == "#")
}

func (b *broker) subscribers(topic string) (clients []*client) {
	b.Lock()
	defer b.Unlock()
	// If wildcards were not supported, then it's rather simple: return b.subs[topic]
	for t, c := range b.subs {
		if b.match(topic, t) {
			clients = append(clients, c...)
		}
	}
	return clients
}

func (b *broker) enqueue(c *client, brokerID, clientID int) {
	b.Lock()
	defer b.Unlock()
	b.pending[brokerID] = pendingMsg{id: clientID, c: c}
}

func (b *broker) dequeue(brokerID int) (c *client, id int) {
	b.Lock()
	defer b.Unlock()
	if p, ok := b.pending[brokerID]; ok {
		delete(b.pending, brokerID)
		return p.c, p.id
	}
	return nil, 0
}

func (b *broker) Publish(topic string, payload []byte) {
	go func() {
		for _, sub := range b.subscribers(topic) {
			b.Lock()
			topicAlias, ok := sub.pubAlias[topic]
			b.Unlock()
			if !ok {
				topicAlias = topic
			}
			sub.publish(0x30, -1, topicAlias, payload)
		}
	}()
}

func (c *client) run() {
	const (
		connect     = 1
		connack     = 2
		publish     = 3
		puback      = 4
		subscribe   = 8
		suback      = 9
		unsubscribe = 10
		unsuback    = 11
		pingreq     = 12
		pingresp    = 13
		disconnect  = 14
	)
	defer func() {
		c.conn.Close()
		if c.broker.closeHook != nil {
			c.broker.closeHook(c.id, c.username, c.password)
		}
	}()
	r := bufio.NewReader(c.conn)
	for {
		header, data, err := c.readPacket(r)
		if err != nil {
			return
		}
		switch header >> 4 {
		case connect:
			if len(data) < 2 {
				return
			}
			protoLen := int(data[0])*256 + int(data[1])
			if len(data) < protoLen+6 {
				return
			}
			proto := string(data[2 : 2+protoLen])
			if proto != "MQTT" && proto != "MQIsdp" {
				c.writePacket(connack<<4, []byte{0, 1})
				return
			}
			flags := data[protoLen+3]
			if c.broker.auth != nil && (flags&0xfc) != 0xc0 {
				c.writePacket(connack<<4, []byte{0, 5})
				return
			}
			data := data[protoLen+6:]
			if c.broker.auth != nil {
				c.id, c.username, c.password = "", "", ""
				for _, s := range []*string{&c.id, &c.username, &c.password} {
					if len(data) < 2 {
						return
					}
					n := int(data[0])*256 + int(data[1])
					if len(data) < 2+n {
						return
					}
					*s = string(data[2 : 2+n])
					data = data[2+n:]
				}
				if err := c.broker.auth(c.id, c.username, c.password); err != nil {
					c.writePacket(connack<<4, []byte{0, 4})
					return
				}
			}
			c.writePacket(connack<<4, []byte{0, 0})
		case publish:
			if len(data) < 2 {
				return
			}
			qos := header & (3 << 1) >> 1
			retain := header&1 != 0
			if qos > 1 || retain {
				return
			}
			size := int(data[0])<<8 | int(data[1])
			if len(data) < 2+size {
				return
			}
			topic := string(data[2 : 2+size])
			payload := data[2+size:]
			msgID := -1
			if qos == 1 {
				if len(data) < 4+size {
					return
				}
				cid := int(data[2+size])*256 + int(data[3+size])
				msgID = int(atomic.AddUint32(&c.broker.msgID, 1) & 0xffff)
				c.broker.enqueue(c, msgID, cid)
				payload = data[4+size:]
			}
			if c.broker.publishHook != nil {
				topic, payload, err = c.broker.publishHook(c.id, c.username, c.password, topic, payload)
				if err != nil {
					return
				}
			}
			for _, sub := range c.broker.subscribers(topic) {
				topicAlias, ok := sub.pubAlias[topic]
				if !ok {
					topicAlias = topic
				}
				sub.publish(header, msgID, topicAlias, payload)
			}
		case subscribe:
			if len(data) < 2 {
				return
			}
			hi := data[0]
			lo := data[1]
			data = data[2:]
			for len(data) > 0 {
				if len(data) < 2 {
					c.writePacket(suback<<4, []byte{hi, lo, 0x80})
					return
				}
				n := int(data[0])*256 + int(data[1])
				if len(data) < 3+n {
					c.writePacket(suback<<4, []byte{hi, lo, 0x80})
					return
				}
				topic := string(data[2 : 2+n])
				qos := data[2+n]
				if qos > 1 {
					c.writePacket(suback<<4, []byte{hi, lo, 0x80})
					return
				}
				if c.broker.subscribeHook != nil {
					origTopic := topic
					topic, err = c.broker.subscribeHook(c.id, c.username, c.password, topic)
					c.subAlias[origTopic] = topic
					c.pubAlias[topic] = origTopic
				}
				c.broker.sub(c, topic)
				data = data[3+n:]
			}
			c.writePacket(suback<<4, []byte{hi, lo, 0})
		case unsubscribe:
			if len(data) < 2 {
				return
			}
			hi := data[0]
			lo := data[1]
			data = data[2:]
			for len(data) > 0 {
				if len(data) < 2 {
					return
				}
				n := int(data[0])*256 + int(data[1])
				if len(data) < 2+n {
					return
				}
				topic := string(data[2 : 2+n])
				topicAlias, ok := c.subAlias[topic]
				if !ok {
					topicAlias = topic
				}
				c.broker.unsub(c, topicAlias)
				data = data[2+n:]
			}
			c.writePacket(unsuback<<4, []byte{hi, lo})
		case puback:
			bid := int(data[0])*256 + int(data[1])
			pub, id := c.broker.dequeue(bid)
			pub.writePacket(puback<<4, []byte{byte(id >> 8), byte(id)})
		case pingreq:
			c.writePacket(pingresp<<4, nil)
		case disconnect:
			return
		}
	}
}

func (c *client) readPacket(r *bufio.Reader) (byte, []byte, error) {
	header, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	size, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, nil, err
	}
	data := make([]byte, size)
	if n, err := io.ReadFull(r, data); err != nil {
		return 0, nil, err
	} else if uint64(n) != size {
		return 0, nil, fmt.Errorf("short read: expected %d but got %d", size, n)
	} else {
		return header, data, nil
	}
}

func (c *client) writePacket(header byte, payload []byte) error {
	buf := make([]byte, binary.MaxVarintLen64+1)
	buf[0] = header
	n := binary.PutUvarint(buf[1:], uint64(len(payload)))
	_, err := c.conn.Write(append(buf[:n+1], payload...))
	return err
}

func (c *client) publish(header byte, msgID int, topic string, payload []byte) error {
	msg := payload
	if msgID >= 0 {
		msg = append([]byte{byte(msgID >> 8), byte(msgID)}, payload...)
	}
	msg = append([]byte(topic), msg...)
	msg = append([]byte{byte(len(topic) >> 8), byte(len(topic))}, msg...)
	return c.writePacket(header, msg)
}
