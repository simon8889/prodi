package hub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"prodi/api/internal/protocol"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 8 << 10
	sendBuffer     = 64
)

var (
	ErrClientNotFound = errors.New("client not found")
	ErrSendTimeout    = errors.New("send timeout")
)

type Handler func(ctx context.Context, clientID uint64, inbound protocol.Inbound)

type Hooks struct {
	OnConnect    func(ctx context.Context, clientID uint64, kind protocol.ClientKind)
	OnDisconnect func(ctx context.Context, kind protocol.ClientKind)
}

type Hub struct {
	mu       sync.RWMutex
	clients  map[uint64]*Client
	nextID   atomic.Uint64
	outbound chan []byte
	closed   atomic.Bool

	hooks     Hooks
	onMessage Handler
	logger    *slog.Logger
}

func New(logger *slog.Logger, onMessage Handler) *Hub {
	return &Hub{
		clients:   make(map[uint64]*Client),
		outbound:  make(chan []byte, 256),
		onMessage: onMessage,
		logger:    logger,
	}
}

func (h *Hub) SetHooks(hooks Hooks) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hooks = hooks
}

func (h *Hub) SetMessageHandler(handler Handler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onMessage = handler
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
		case data := <-h.outbound:
			h.fanout(data)
		}
	}
}

func (h *Hub) shutdown() {
	if !h.closed.CompareAndSwap(false, true) {
		return
	}
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.clients = make(map[uint64]*Client)
	hooks := h.hooks
	h.mu.Unlock()

	for _, c := range clients {
		c.close()
		if hooks.OnDisconnect != nil {
			hooks.OnDisconnect(context.Background(), c.kind)
		}
	}
}

func (h *Hub) fanout(data []byte) {
	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.enqueue(data)
	}
}

func (h *Hub) Broadcast(eventType string, payload any, at time.Time) {
	data, err := protocol.Encode(eventType, payload, at)
	if err != nil {
		h.logger.Error("encode broadcast", "type", eventType, "error", err)
		return
	}
	h.BroadcastRaw(data)
}

func (h *Hub) BroadcastRaw(data []byte) {
	if h.closed.Load() {
		return
	}
	select {
	case h.outbound <- data:
	default:
		h.mu.RLock()
		count := len(h.clients)
		h.mu.RUnlock()
		h.logger.Warn("broadcast buffer full, dropping message", "clients", count, "bytes", len(data))
	}
}

func (h *Hub) SendTo(ctx context.Context, clientID uint64, eventType string, payload any, at time.Time) error {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()
	if !ok {
		return ErrClientNotFound
	}
	return client.sendTo(ctx, eventType, payload, at)
}

type ClientCounts struct {
	Total   int `json:"total"`
	Web     int `json:"web"`
	Devices int `json:"devices"`
}

func (h *Hub) Counts() ClientCounts {
	h.mu.RLock()
	defer h.mu.RUnlock()
	counts := ClientCounts{}
	for _, c := range h.clients {
		counts.Total++
		if c.kind == protocol.ClientKindDevice {
			counts.Devices++
		} else {
			counts.Web++
		}
	}
	return counts
}

func (h *Hub) Register(ctx context.Context, conn *websocket.Conn, kind protocol.ClientKind, identity string) {
	client := &Client{
		id:     h.nextID.Add(1),
		conn:   conn,
		kind:   kind,
		send:   make(chan []byte, sendBuffer),
		hub:    h,
		logger: h.logger.With("client", identity, "kind", string(kind)),
	}

	h.mu.Lock()
	if h.closed.Load() {
		h.mu.Unlock()
		_ = conn.Close(websocket.StatusGoingAway, "server shutting down")
		return
	}
	h.clients[client.id] = client
	hooks := h.hooks
	h.mu.Unlock()

	client.logger.Info("client connected")
	if hooks.OnConnect != nil {
		hooks.OnConnect(ctx, client.id, kind)
	}

	defer func() {
		h.mu.Lock()
		delete(h.clients, client.id)
		current := h.hooks
		h.mu.Unlock()
		client.close()
		client.logger.Info("client disconnected")
		if current.OnDisconnect != nil {
			current.OnDisconnect(context.Background(), client.kind)
		}
	}()

	go client.writePump()
	client.readPump(ctx, h.onMessage)
}

type Client struct {
	id     uint64
	conn   *websocket.Conn
	kind   protocol.ClientKind
	hub    *Hub
	logger *slog.Logger

	mu     sync.Mutex
	send   chan []byte
	closed bool
}

func (c *Client) ID() uint64 {
	return c.id
}

func (c *Client) enqueue(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- data:
	default:
		c.logger.Warn("client send buffer full, closing connection")
		c.closed = true
		close(c.send)
		go c.conn.Close(websocket.StatusPolicyViolation, "send buffer full")
	}
}

func (c *Client) sendTo(ctx context.Context, eventType string, payload any, at time.Time) error {
	data, err := protocol.Encode(eventType, payload, at)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClientNotFound
	}
	select {
	case c.send <- data:
		return nil
	default:
		return ErrSendTimeout
	}
}

func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
	go c.conn.Close(websocket.StatusNormalClosure, "")
}

func (c *Client) readPump(ctx context.Context, handler Handler) {
	c.conn.SetReadLimit(maxMessageSize)

	for {
		msgType, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		if msgType != websocket.MessageText {
			continue
		}

		var inbound protocol.Inbound
		if err := json.Unmarshal(data, &inbound); err != nil {
			c.enqueuePayload(protocol.ServerError, protocol.ErrorPayload{
				Code:    "invalid_json",
				Message: "message is not valid JSON",
			})
			continue
		}

		if inbound.Type == protocol.ClientPing {
			c.enqueuePayload(protocol.ServerHello, protocol.HelloPayload{
				Kind:      c.kind,
				ServerTime: time.Now().UTC(),
			})
			continue
		}
		if handler != nil {
			handler(ctx, c.id, inbound)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				_ = c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			writeCtx, writeCancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Write(writeCtx, websocket.MessageText, data)
			writeCancel()
			if err != nil {
				c.logger.Debug("write failed", "error", err)
				c.close()
				return
			}
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				c.logger.Debug("ping failed", "error", err)
				c.close()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) enqueuePayload(eventType string, payload any) {
	data, err := protocol.Encode(eventType, payload, time.Now().UTC())
	if err != nil {
		c.logger.Error("encode payload", "type", eventType, "error", err)
		return
	}
	c.enqueue(data)
}
