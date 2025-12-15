package neural

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	models "MicroserviceWebsocket/internal/domain"

	"github.com/gorilla/websocket"
)

// type Client struct {
// 	conn    *websocket.Conn
// 	url     string
// 	isReady bool
// 	timeout time.Duration
// 	mutex   sync.Mutex
// }

type Client struct {
	url     string
	timeout time.Duration

	// состояние соединения
	mu       sync.Mutex
	conn     *websocket.Conn
	isReady  bool
	stopConn context.CancelFunc

	// один writer
	writeCh chan any

	// ожидания по uuid
	pendingMu sync.Mutex
	pending   map[string]chan result

	// чтобы не запускать параллельно несколько reconnect
	reconnectMu sync.Mutex
}

type result struct {
	resp models.Response
	err  error
}

type typeMsg struct {
	Type string `json:"type"`
}

func NewClient(neuralURL string, timeout time.Duration) *Client {
	c := &Client{
		url:     neuralURL,
		timeout: timeout,
		writeCh: make(chan any, 256),
		pending: make(map[string]chan result),
	}
	go c.connectLoop()
	return c
}

// Close — корректно останавливает клиент и завершает pending
func (c *Client) Close() {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	c.mu.Lock()
	cancel := c.stopConn
	conn := c.conn
	c.conn = nil
	c.isReady = false
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if conn != nil {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}

	c.failAllPending(errors.New("client closed"))
}

// обработка ошибок при подклоючении и retry connect
func (c *Client) connectLoop() {
	for {
		if err := c.connectOnce(); err != nil {
			log.Printf("Failed to connect to neural service: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		return
	}
}

// connect
func (c *Client) connectOnce() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	// если кто-то уже подключил — закрываем новое
	if c.conn != nil {
		c.mu.Unlock()
		cancel()
		_ = conn.Close()
		return nil
	}
	c.conn = conn
	c.isReady = true
	c.stopConn = cancel
	c.mu.Unlock()

	log.Printf("Connected to neural service: %s", c.url)

	go c.writeLoop(ctx, conn)
	go c.readLoop(ctx, conn)

	return nil
}

func (c *Client) reconnect(reason error) {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	// закрываем старое соединение и останавливаем loops
	c.mu.Lock()
	cancel := c.stopConn
	conn := c.conn
	c.conn = nil
	c.isReady = false
	c.stopConn = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}

	// валим все ожидающие
	if reason == nil {
		reason = errors.New("connection lost")
	}
	c.failAllPending(reason)

	log.Printf("Neural connection lost (%v), reconnecting...", reason)

	// пытаемся подключиться заново
	for {
		if err := c.connectOnce(); err != nil {
			log.Printf("Reconnect failed: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		return
	}
}

// собирает запросы и делает фактическое write по ws сокету
func (c *Client) writeLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.writeCh:
			// _ = conn.SetWriteDeadline(time.Now().Add(c.timeout))
			if err := conn.WriteJSON(msg); err != nil {
				go c.reconnect(fmt.Errorf("write error: %w", err))
				return
			}
		}
	}
}

// здесь надо добавить чтобы он создал таблицу в которой хранится
// читает все сообщения с ws соединений и сам уже делит на ping и promt
func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn) {
	const op = "readLoop"

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// _ = conn.SetReadDeadline(time.Now().Add(c.timeout)

			// читаем raw JSON, чтобы отличить ping от ChatResponse
			var raw map[string]any
			if err := conn.ReadJSON(&raw); err != nil {
				go c.reconnect(fmt.Errorf("read error: %w", err))
				return
			}

			// ping -> pong
			if t, ok := raw["type"].(string); ok && t == "ping" {
				// отправляем pong через writer
				select {
				case c.writeCh <- typeMsg{Type: "pong"}:
				default:
					// если очередь забита — всё равно не падаем, но это сигнал перегруза
					log.Printf("write queue is full, dropping pong")
				}
				continue
			}

			b, _ := json.Marshal(raw)
			var resp models.Response
			if err := json.Unmarshal(b, &resp); err != nil {
				// если прилетел неожиданный формат — не роняем соединение,
				// но логируем и продолжаем
				log.Printf("unexpected message from server: %s", string(b))
				continue
			}

			if resp.UUID == "" {
				continue
			}

			c.pendingMu.Lock()
			ch := c.pending[resp.UUID]
			if ch != nil {
				delete(c.pending, resp.UUID)
			}
			c.pendingMu.Unlock()

			log.Printf("[%s] <- response uuid=%s created_at=%s text=%q",
				op,
				resp.UUID,
				resp.CreatedAt,
				trimLong(resp.Response),
			)

			if ch != nil {
				ch <- result{resp: resp, err: nil}
				close(ch)
			}
		}
	}
}

// ===== public API =====

// ProcessSingle отправляет один запрос и ждёт ответ по uuid
func (c *Client) ProcessSingle(request models.Request) (models.Response, error) {

	const op = "ProcessSingle"
	// ожидаем наличие соединения (быстро)
	c.mu.Lock()
	ready := c.isReady && c.conn != nil
	c.mu.Unlock()
	if !ready {
		return models.Response{}, errors.New("neural service not available")
	}

	uuid := request.UUID

	// регистрируем ожидание
	waitCh := make(chan result, 1)

	c.pendingMu.Lock()
	// если uuid уже в ожидании — это логическая ошибка у вызывающего кода
	if _, exists := c.pending[uuid]; exists {
		c.pendingMu.Unlock()
		return models.Response{}, fmt.Errorf("uuid already pending: %s", uuid)
	}
	c.pending[uuid] = waitCh
	c.pendingMu.Unlock()

	// формируем payload для gateway:
	// request.Prompt -> message
	payload := models.Request{
		UUID:      request.UUID,
		ModelName: request.ModelName,
		Message:   request.Message,
	}

	// отправляем через writer-очередь
	select {
	case c.writeCh <- payload:
	default:
		// очередь переполнена — снимаем pending и возвращаем ошибку
		c.pendingMu.Lock()
		delete(c.pending, uuid)
		c.pendingMu.Unlock()
		return models.Response{}, errors.New("write queue is full")
	}

	// ждём ответ/ошибку/таймаут
	select {
	case r := <-waitCh:
		if r.err != nil {
			return models.Response{}, r.err
		}
		// адаптируем обратно в ваш models.Response
		return models.Response{
			UUID:      r.resp.UUID,
			Response:  r.resp.Response,
			CreatedAt: r.resp.CreatedAt,
		}, nil

	case <-time.After(c.timeout):
		// снимаем pending (чтобы не утекало)
		c.pendingMu.Lock()
		if ch := c.pending[uuid]; ch != nil {
			delete(c.pending, uuid)
			close(ch)
		}
		c.pendingMu.Unlock()

		log.Printf("[%s] -> request uuid=%s model=%s text=%q",
			op,
			payload.UUID,
			payload.ModelName,
			trimLong(payload.Message),
		)

		return models.Response{}, errors.New("timeout waiting neural response")
	}
}

func (c *Client) failAllPending(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	for uuid, ch := range c.pending {
		delete(c.pending, uuid)
		select {
		case ch <- result{err: err}:
		default:
		}
		close(ch)
	}
}

func trimLong(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "...(truncated)"
}
