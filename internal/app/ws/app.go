package ws

import (
	"context"
	"fmt"

	"MicroserviceWebsocket/internal/config"
	"MicroserviceWebsocket/internal/server/handlers"
	"net/http"
	"time"

	httpHandlers "MicroserviceWebsocket/internal/server/http"

	"golang.org/x/exp/slog"
)

type App struct {
	log       *slog.Logger
	server    *http.Server
	wsHandler *handlers.WebSocketHandler
	httpAPI   *httpHandlers.API
	config    *config.Config
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Для теста можно разрешить всем. Потом сузишь до нужного origin.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func New(
	log *slog.Logger,
	cfg *config.Config,
	wsHandler *handlers.WebSocketHandler,
	httpAPI *httpHandlers.API,
) *App {
	// Создаем HTTP сервер с WebSocket хендлером
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler.HandleConnection)
	mux.HandleFunc("/chats", httpAPI.Chats)     // POST /chats, GET /chats?user_id=...
	mux.HandleFunc("/chats/", httpAPI.ChatByID) // GET /chats/{id}/messages, DELETE /chats/{id}
	mux.HandleFunc("/messages/", httpAPI.MessageByID)
	mux.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:         cfg.WEBSOCKET.URLWS,
		Handler:      withCORS(mux),
		ReadTimeout:  cfg.WEBSOCKET.Timeout,
		WriteTimeout: cfg.WEBSOCKET.Timeout,
	}

	return &App{
		log:       log,
		server:    server,
		wsHandler: wsHandler,
		httpAPI:   httpAPI,
		config:    cfg,
	}
}

// MustRun запускает сервер или паникует при ошибке
func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

// Run запускает WebSocket сервер
func (a *App) Run() error {
	const op = "wsapp.Run"

	a.log.Info("starting WebSocket server",
		slog.String("addr", a.server.Addr),
		slog.String("env", a.config.ENV),
	)

	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// Stop gracefully останавливает сервер
func (a *App) Stop() error {
	const op = "wsapp.Stop"

	a.log.Info("stopping WebSocket server")

	// Graceful shutdown с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("WebSocket server stopped")
	return nil
}

// healthHandler для проверки здоровья сервиса
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok", "service": "ws-service"}`))
}
