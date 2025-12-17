package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	models "MicroserviceWebsocket/internal/domain"
	_ "MicroserviceWebsocket/internal/services/batch"
	"MicroserviceWebsocket/internal/services/neural"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	pongWait   = 20 * time.Second
	pingPeriod = 15 * time.Second
)

type Storage interface {
	InsertUserMessage(ctx context.Context, chatUUID, messageUUID, content string) error
	InsertBotMessage(ctx context.Context, chatUUID, messageUUID, content, replyToUUID string) error
}

type WebSocketHandler struct {
	neuralClient *neural.Client
	storage      Storage
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// На время разработки можно так (разрешаем всем)
		return true
	},
}

func NewWebSocketHandler(neuralClient *neural.Client, storage Storage) *WebSocketHandler {
	return &WebSocketHandler{neuralClient: neuralClient, storage: storage}
}

func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	const op = "WebSocketHandler.HandleConnection"

	// 1. Всегда проверяем ошибку Upgrade
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("%s: upgrade error: %v", op, err)
		return
	}
	defer conn.Close()

	// проверка понгов
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Горутина для пингов
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			if err := conn.WriteControl(
				websocket.PingMessage,
				[]byte("ping"),
				time.Now().Add(5*time.Second),
			); err != nil {
				log.Println("ping error, closing:", err)
				conn.Close()
				return
			}
		}
	}()

	// 2. Читаем сообщения
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("%s: read error: %v", op, err)
			return
		}

		// 3. Обработка сообщения
		// go h.handleMessage(conn, message)
		h.handleMessage(conn, message)
	}
}

// func (h *WebSocketHandler) handleMessage(conn *websocket.Conn, msg []byte) {
// 	const op = "WebSocketHandler.handleMessage"
// 	// Валидация формата
// 	request, err := validateMessage(string(msg))
// 	if err != nil {
// 		log.Print(fmt.Errorf("%w: %s", err, op))
// 		return
// 	}

// 	// Аутентификация через Auth сервис
// 	// здесь userid не учитываем так как происходит проверка внутри на то что пользователь с данным userid существует в бд
// 	// _, err = h.authClient.ValidateToken(context.Background(), request.Token)
// 	// if err != nil {
// 	// 	log.Print(fmt.Errorf("%w: %s", err, op))
// 	// 	return
// 	// }

// 	// Передача в сервисный слой
// 	result, err := h.neuralClient.ProcessSingle(request)
// 	fmt.Println(request)
// 	if err != nil {
// 		log.Print(fmt.Errorf("%w: %s", err, op))
// 		return
// 	}

// 	conn.WriteJSON(result)
// }

func (h *WebSocketHandler) handleMessage(conn *websocket.Conn, msg []byte) {
	const op = "WebSocketHandler.handleMessage"

	request, err := validateMessage(string(msg))
	if err != nil {
		log.Print(fmt.Errorf("%w: %s", err, op))
		return
	}

	// validate UUIDs
	if _, err := uuid.Parse(request.ChatUUID); err != nil {
		_ = conn.WriteJSON(map[string]any{"error": "validation_error", "msg": "chat_uuid must be uuid"})
		return
	}
	if _, err := uuid.Parse(request.UUID); err != nil {
		_ = conn.WriteJSON(map[string]any{"error": "validation_error", "msg": "uuid must be uuid"})
		return
	}

	// 1) save user message
	if err := h.storage.InsertUserMessage(context.Background(), request.ChatUUID, request.UUID, request.Message); err != nil {
		_ = conn.WriteJSON(map[string]any{"error": "db_error", "msg": err.Error()})
		return
	}

	// 2) neural
	result, err := h.neuralClient.ProcessSingle(request)
	if err != nil {
		_ = conn.WriteJSON(map[string]any{"error": "neural_error", "msg": err.Error()})
		return
	}

	// 3) save bot message
	botUUID := uuid.NewString()
	if err := h.storage.InsertBotMessage(context.Background(), request.ChatUUID, botUUID, result.Response, request.UUID); err != nil {
		_ = conn.WriteJSON(map[string]any{"error": "db_error", "msg": err.Error()})
		return
	}

	resp := models.WSBotMessage{
		Type:            "bot_message",
		ChatUUID:        request.ChatUUID,
		UserMessageUUID: request.UUID,
		BotMessageUUID:  botUUID,
		Response:        result.Response,
		CreatedAt:       result.CreatedAt,
	}

	if err := conn.WriteJSON(resp); err != nil {
		log.Printf("write ws json error: %v", err)
	}
}

func validateMessage(msgStr string) (models.Request, error) {
	var request models.Request
	err := json.Unmarshal([]byte(msgStr), &request)
	if err != nil {
		return models.Request{}, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	return request, nil
}
