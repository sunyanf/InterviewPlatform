package realtime

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second // 单次写帧超时
	pongWait       = 60 * time.Second // 等待 pong 的最长时间（读空闲超时）
	pingPeriod     = 50 * time.Second // 服务端 ping 周期（< pongWait）
	sendBufferSize = 64               // 单连接待发送帧缓冲
	closeReplaced  = 4000             // 自定义 close code：被新连接顶替
	closeAppError  = 4001             // 自定义 close code：服务端主动关闭
)

// Client 单个 WebSocket 连接。
// 所有帧只能经 send channel 由 writePump 写出（gorilla 要求单 goroutine 写）。
type Client struct {
	sessionID string
	userID    string
	conn      *websocket.Conn

	send     chan []byte
	shutdown chan struct{} // 通知 writePump 发 close frame 并退出
	closed   chan struct{} // writePump 退出（conn 已关闭）

	closeOnce sync.Once
	reason    string
}

// newClient 创建连接对象并启动写泵
func newClient(sessionID, userID string, conn *websocket.Conn) *Client {
	c := &Client{
		sessionID: sessionID,
		userID:    userID,
		conn:      conn,
		send:      make(chan []byte, sendBufferSize),
		shutdown:  make(chan struct{}),
		closed:    make(chan struct{}),
	}
	go c.writePump()
	return c
}

// sendFrame 入队一帧；缓冲已满（慢消费者）或连接关闭中则返回 false 并触发关闭
func (c *Client) sendFrame(b []byte) bool {
	select {
	case c.send <- b:
		return true
	case <-c.shutdown:
		return false
	default:
		c.close("send buffer full")
		return false
	}
}

// close 请求关闭连接（幂等），由 writePump 完成 close frame 与底层关闭
func (c *Client) close(reason string) {
	c.closeOnce.Do(func() {
		c.reason = reason
		close(c.shutdown)
	})
}

// writePump 唯一的 conn 写 goroutine：消费 send、周期 ping、响应关闭请求
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
		close(c.closed)
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(closeAppError, "server shutdown"))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.shutdown:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(closeReplaced, c.reason))
			return
		}
	}
}

// Hub 单个面试会话的连接中枢。同一 session 只允许一个活跃连接：
// 新连接顶替旧连接（断线重连场景下旧连接可能仍处于半开状态）。
type Hub struct {
	sessionID string

	mu     sync.Mutex
	client *Client
}

// replace 注册新连接，返回被顶替的旧连接（可能为 nil）
func (h *Hub) replace(c *Client) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	old := h.client
	h.client = c
	return old
}

// clear 仅当当前连接仍是 c 时清空（旧连接延迟退出时不会误删新连接）
func (h *Hub) clear(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.client == c {
		h.client = nil
	}
}

// HubManager 按 sessionID 管理 Hub 生命周期，最后一个连接退出后回收 Hub
type HubManager struct {
	mu   sync.Mutex
	hubs map[string]*Hub
}

// NewHubManager 创建 HubManager
func NewHubManager() *HubManager {
	return &HubManager{hubs: make(map[string]*Hub)}
}

// acquire 获取或创建 session 对应的 Hub
func (m *HubManager) acquire(sessionID string) *Hub {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := m.hubs[sessionID]
	if h == nil {
		h = &Hub{sessionID: sessionID}
		m.hubs[sessionID] = h
	}
	return h
}

// release 连接彻底退出时调用；Hub 上已无连接则回收（双重检查防止与新连接竞态）
func (m *HubManager) release(sessionID string, h *Hub) {
	h.mu.Lock()
	if h.client != nil {
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hubs[sessionID] != h {
		return
	}
	h.mu.Lock()
	empty := h.client == nil
	h.mu.Unlock()
	if empty {
		delete(m.hubs, sessionID)
	}
}
