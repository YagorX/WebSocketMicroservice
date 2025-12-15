package ws

import (
	"context"
	"fmt"

	"MicroserviceWebsocket/internal/config"
	"MicroserviceWebsocket/internal/server/handlers"
	"net/http"
	"time"

	"golang.org/x/exp/slog"
)

type App struct {
	log       *slog.Logger
	server    *http.Server
	wsHandler *handlers.WebSocketHandler
	config    *config.Config
}

func New(
	log *slog.Logger,
	cfg *config.Config,
	wsHandler *handlers.WebSocketHandler,
) *App {
	// Создаем HTTP сервер с WebSocket хендлером
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler.HandleConnection)
	mux.HandleFunc("/health", healthHandler)

	server := &http.Server{
		Addr:         cfg.WEBSOCKET.URLWS,
		Handler:      mux,
		ReadTimeout:  cfg.WEBSOCKET.Timeout,
		WriteTimeout: cfg.WEBSOCKET.Timeout,
	}

	return &App{
		log:       log,
		server:    server,
		wsHandler: wsHandler,
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
