package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	models "MicroserviceWebsocket/internal/domain"
	_ "MicroserviceWebsocket/internal/services/batch"
	"MicroserviceWebsocket/internal/services/neural"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 20 * time.Second
	pingPeriod = 15 * time.Second
)

type WebSocketHandler struct {
	// authClient   auth.Client
	neuralClient *neural.Client
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// На время разработки можно так (разрешаем всем)
		return true
	},
}

// func NewWebSocketHandler(authClient auth.Client, neuralClient *neural.Client) *WebSocketHandler {
// 	return &WebSocketHandler{
// 		authClient:   authClient,
// 		neuralClient: neuralClient,
// 	}
// }

func NewWebSocketHandler(neuralClient *neural.Client) *WebSocketHandler {
	return &WebSocketHandler{
		neuralClient: neuralClient,
	}
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

func (h *WebSocketHandler) handleMessage(conn *websocket.Conn, msg []byte) {
	const op = "WebSocketHandler.handleMessage"
	// Валидация формата
	request, err := validateMessage(string(msg))
	if err != nil {
		log.Print(fmt.Errorf("%w: %s", err, op))
		return
	}

	// Аутентификация через Auth сервис
	// здесь userid не учитываем так как происходит проверка внутри на то что пользователь с данным userid существует в бд
	// _, err = h.authClient.ValidateToken(context.Background(), request.Token)
	// if err != nil {
	// 	log.Print(fmt.Errorf("%w: %s", err, op))
	// 	return
	// }

	// Передача в сервисный слой
	result, err := h.neuralClient.ProcessSingle(request)
	fmt.Println(request)
	if err != nil {
		log.Print(fmt.Errorf("%w: %s", err, op))
		return
	}

	conn.WriteJSON(result)
}

func validateMessage(msgStr string) (models.Request, error) {
	var request models.Request
	err := json.Unmarshal([]byte(msgStr), &request)
	if err != nil {
		return models.Request{}, fmt.Errorf("JSON unmarshal error: %v", err)
	}

	return request, nil
}
