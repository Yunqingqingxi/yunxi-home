package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

const (
	// writeWait is the timeout for writing a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// pingPeriod is the interval between pings. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum message size allowed from the peer.
	maxMessageSize = 8192
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WSMessage WebSocket 消息格式
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// ptyIO 平台相关的 PTY/进程 I/O 接口
type ptyIO interface {
	io.ReadWriteCloser
	Resize(rows, cols int) error
}

// TerminalHandler Web 终端处理器
type TerminalHandler struct {
	enabled   bool
	adminOnly bool
}

// NewHandler 创建终端处理器
func NewHandler(enabled, adminOnly bool) *TerminalHandler {
	return &TerminalHandler{enabled: enabled, adminOnly: adminOnly}
}

// Handle 处理 WebSocket 升级
func (h *TerminalHandler) Handle(c echo.Context) error {
	if !h.enabled {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "终端未启用"})
	}
	if h.adminOnly {
		user := c.Get("user")
		if user == nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "仅管理员可使用终端"})
		}
		// Echo JWT 中间件存储 *jwt.Token; 通过接口提取 role
		type hasClaims interface{ Claims() interface{} }
		if t, ok := user.(hasClaims); ok {
			raw := t.Claims()
			// 尝试 json 序列化后提取 role 字段 (兼容所有 Claims 类型)
			if b, err := json.Marshal(raw); err == nil {
				var m map[string]interface{}
				if json.Unmarshal(b, &m) == nil {
					if role, _ := m["role"].(string); role != "admin" {
						return c.JSON(http.StatusForbidden, map[string]string{"error": "仅管理员可使用终端"})
					}
				}
			}
		}
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		slog.Error("WebSocket 升级失败", "error", err)
		return err
	}

	// Set initial read deadline and pong handler to detect dead connections.
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	session := &Session{
		conn:   conn,
		done:   make(chan struct{}),
		closed: false,
	}
	session.start()
	return nil
}

// Session 终端会话
type Session struct {
	conn   *websocket.Conn
	pty    ptyIO
	done   chan struct{}
	mu     sync.Mutex
	closed bool
}

func (s *Session) start() {
	pty, err := startShell()
	if err != nil {
		slog.Error("启动 Shell 失败", "error", err)
		s.conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n启动 Shell 失败: %v\r\n", err)))
		s.conn.Close()
		return
	}
	s.pty = pty

	go s.keepAlive()
	go s.readLoop()
	go s.writeLoop()
}

// readLoop reads from the PTY and writes to the WebSocket.
func (s *Session) readLoop() {
	defer s.cleanup()
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			// Set write deadline before writing to detect client that stopped reading.
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if writeErr := s.conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
				slog.Debug("写入 WebSocket 失败", "error", writeErr)
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				slog.Debug("读取 Shell 输出错误", "error", err)
			}
			return
		}
	}
}

// writeLoop reads from the WebSocket and writes to the PTY.
// ReadMessage will timeout after pongWait if the client stops sending pongs.
func (s *Session) writeLoop() {
	defer s.cleanup()
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Debug("WebSocket 读取错误", "error", err)
			}
			return
		}
		var wsMsg WSMessage
		if json.Unmarshal(msg, &wsMsg) == nil && wsMsg.Type == "input" {
			var input string
			json.Unmarshal(wsMsg.Data, &input)
			s.pty.Write([]byte(input))
		} else if wsMsg.Type == "resize" {
			var dims struct{ Rows, Cols int }
			json.Unmarshal(wsMsg.Data, &dims)
			s.pty.Resize(dims.Rows, dims.Cols)
		} else {
			s.pty.Write(msg)
		}
	}
}

func (s *Session) cleanup() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.done)
	}
	s.mu.Unlock()
	if s.pty != nil {
		s.pty.Close()
	}
	s.conn.Close()
}

// keepAlive sends periodic pings and enforces the pong deadline.
// If the client fails to respond with a pong within pongWait,
// writeLoop's ReadMessage will return a timeout error and the session closes.
func (s *Session) keepAlive() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			done := s.closed
			s.mu.Unlock()
			if done {
				return
			}
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Debug("发送 ping 失败，关闭会话", "error", err)
				s.cleanup()
				return
			}
		case <-s.done:
			return
		}
	}
}