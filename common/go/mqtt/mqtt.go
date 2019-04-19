package mqtt

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/websocket"
)

type AuthFunc func(c *Client) error
type CloseFunc func(c *Client) error
type PublishFunc func(id, username, password, topic string, payload []byte) (string, []byte, error)
type SubscribeFunc func(c *Client, topic string) (string, error)

type Hooks struct {
	Auth      AuthFunc
	Close     CloseFunc
	Publish   PublishFunc
	Subscribe SubscribeFunc
}

// Accepter waits for and returns the next connection to the listener
type Accepter interface {
	Accept() (net.Conn, error)
}

// Broker is a Pub/Sub message forwarder for MQTT protocol.
type Broker interface {
	Run(l Accepter) error
	Publish(topic string, payload []byte)
	PublishEx(header byte, msgID int, id, user, pass, topic string, payload []byte)
}

type broker struct {
	sync.Mutex
	msgID   uint32
	subs    map[string][]*Client
	pending map[int]pendingMsg
	hooks   *Hooks
}

type Client struct {
	Broker      *broker
	Conn        net.Conn
	ID          string
	Username    string
	Password    string
	WillTopic   string
	WillMessage string
	subAlias    map[string]string
	pubAlias    map[string]string
}

type pendingMsg struct {
	id int
	c  *Client
}

// NewBroker creates a new MQTT broker with a user-defined authorization function
func NewBroker(hooks *Hooks) Broker {
	if hooks == nil {
		hooks = &Hooks{}
	}
	brk := &broker{
		hooks:   hooks,
		subs:    map[string][]*Client{},
		pending: map[int]pendingMsg{},
	}
	return brk
}

func (b *broker) Run(l Accepter) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		client := &Client{
			Broker:   b,
			Conn:     conn,
			subAlias: map[string]string{},
			pubAlias: map[string]string{},
		}
		go client.run()
	}
}

func (b *broker) sub(c *Client, topics ...string) {
	b.Lock()
	defer b.Unlock()
	for _, topic := range topics {
		b.subs[topic] = append(b.subs[topic], c)
	}
}

func (b *broker) unsub(c *Client, topics ...string) {
	b.Lock()
	defer b.Unlock()
	for _, topic := range topics {
		clients := b.subs[topic]
		newClients := make([]*Client, 0, len(clients))
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
	return len(parts) == len(wild) //|| (len(parts) == len(wild)-1 && wild[len(wild)-1] == "#")
}

func (b *broker) subscribers(topic string) (clients []*Client) {
	b.Lock()
	defer b.Unlock()
	// If wildcards were not supported, then it's rather simple: return b.subs[topic]
	for t, c := range b.subs {
		if b.match(topic, t) {
			// log.Printf("---> [%s] [%s]\n", topic, t)
			clients = append(clients, c...)
		}
	}
	return clients
}

func (b *broker) enqueue(c *Client, brokerID, clientID int) {
	b.Lock()
	defer b.Unlock()
	b.pending[brokerID] = pendingMsg{id: clientID, c: c}
}

func (b *broker) dequeue(brokerID int) (c *Client, id int) {
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
			// log.Println("-->Publish", sub.id, topic, string(payload))
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

func (b *broker) PublishEx(header byte, msgID int, id, user, pass, topic string, payload []byte) {
	origTopic := topic
	if b.hooks.Publish != nil {
		topic, payload, _ = b.hooks.Publish(id, user, pass, topic, payload)
	}
	for _, sub := range b.subscribers(topic) {
		// log.Println("-->PublishEx", sub.id, id, topic, string(payload))
		topicAlias, ok := sub.pubAlias[topic]
		if !ok {
			topicAlias = origTopic
		}
		sub.publish(header, msgID, topicAlias, payload)
	}
}

func (c *Client) run() {
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
		c.Conn.Close()
		if c.Broker.hooks.Close != nil {
			c.Broker.hooks.Close(c)
		}
	}()
	r := bufio.NewReader(c.Conn)
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
			fields := []*string{&c.ID}
			if flags&(1<<2) != 0 {
				fields = append(fields, &c.WillTopic, &c.WillMessage)
			}
			if flags&0x80 != 0 {
				fields = append(fields, &c.Username)
			}
			if flags&0x40 != 0 {
				fields = append(fields, &c.Password)
			}
			data := data[protoLen+6:]
			c.ID, c.Username, c.Password, c.WillTopic, c.WillMessage = "", "", "", "", ""
			for _, s := range fields {
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

			// Make client ID unique by appending some random string to it
			buf := make([]byte, 4)
			rand.Read(buf)
			c.ID += "." + hex.EncodeToString(buf)

			if c.WillTopic != "" {
				defer c.Broker.PublishEx(0x30, -1, c.ID, c.Username, c.Password, c.WillTopic, []byte(c.WillMessage))
			}
			if c.Broker.hooks.Auth != nil {
				if err := c.Broker.hooks.Auth(c); err != nil {
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
				msgID = int(atomic.AddUint32(&c.Broker.msgID, 1) & 0xffff)
				c.Broker.enqueue(c, msgID, cid)
				payload = data[4+size:]
			}
			c.Broker.PublishEx(header, msgID, c.ID, c.Username, c.Password, topic, payload)
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
				if c.Broker.hooks.Subscribe != nil {
					origTopic := topic
					topic, err = c.Broker.hooks.Subscribe(c, topic)
					// log.Printf("SUB: [%s] [%s] [%s]\n", c.ID, origTopic, topic)
					c.subAlias[origTopic] = topic
					c.pubAlias[topic] = origTopic
				}
				c.Broker.sub(c, topic)
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
				c.Broker.unsub(c, topicAlias)
				data = data[2+n:]
			}
			c.writePacket(unsuback<<4, []byte{hi, lo})
		case puback:
			bid := int(data[0])*256 + int(data[1])
			pub, id := c.Broker.dequeue(bid)
			if pub != nil {
				pub.writePacket(puback<<4, []byte{byte(id >> 8), byte(id)})
			}
		case pingreq:
			c.writePacket(pingresp<<4, nil)
		case disconnect:
			return
		}
	}
}

func (c *Client) readPacket(r *bufio.Reader) (byte, []byte, error) {
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

func (c *Client) writePacket(header byte, payload []byte) error {
	buf := make([]byte, binary.MaxVarintLen64+1)
	buf[0] = header
	n := binary.PutUvarint(buf[1:], uint64(len(payload)))
	pkt := append(buf[:n+1], payload...)
	// log.Println("-->", hex.EncodeToString(pkt))
	_, err := c.Conn.Write(pkt)
	return err
}

func (c *Client) publish(header byte, msgID int, topic string, payload []byte) error {
	msg := payload
	if msgID >= 0 {
		msg = append([]byte{byte(msgID >> 8), byte(msgID)}, payload...)
	}
	msg = append([]byte(topic), msg...)
	msg = append([]byte{byte(len(topic) >> 8), byte(len(topic))}, msg...)
	// log.Println("-->pub", c.ID, msgID, topic, string(payload))
	return c.writePacket(header, msg)
}

type WebSocketHandler struct {
	once sync.Once
	c    chan net.Conn
}

func (h *WebSocketHandler) init() {
	h.once.Do(func() { h.c = make(chan net.Conn) })
}

func (h *WebSocketHandler) Accept() (net.Conn, error) {
	h.init()
	return <-h.c, nil
}

type BinaryWebsocketConn struct {
	ws *websocket.Conn
	websocket.Conn
}

func (x *BinaryWebsocketConn) Write(msg []byte) (n int, err error) {
	return n, websocket.Message.Send(x.ws, msg)
}

func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.init()
	websocket.Handler(func(ws *websocket.Conn) {
		h.c <- &BinaryWebsocketConn{ws, *ws}
		<-ws.Request().Context().Done()
	}).ServeHTTP(w, r)
}
