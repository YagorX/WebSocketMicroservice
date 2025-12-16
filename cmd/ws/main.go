package main

import (
	"MicroserviceWebsocket/internal/app/ws"
	"MicroserviceWebsocket/internal/config"
	"MicroserviceWebsocket/internal/lib/logger/handlers/slogpretty"
	"MicroserviceWebsocket/internal/server/handlers"
	"MicroserviceWebsocket/internal/server/http"
	"MicroserviceWebsocket/internal/services/neural"
	"MicroserviceWebsocket/internal/storage/postgresql"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/exp/slog"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	// TODO: инициализировать объект конфига
	cfg := config.MustLoad()

	// TODO: инициализировать логгер
	log := setupLogger(cfg.ENV)

	// TODO: инициализировать приложение (app)
	//инициализация подключения к беку
	neuralClient := neural.NewClient(cfg.NEURALCLIENT.URLNeural, cfg.NEURALCLIENT.Timeout)
	log.Info("Neural service activate")

	//инициализация подключения к auth
	// authClient, err := auth.New(
	// 	context.Background(),
	// 	log,
	// 	cfg.AUTH.URLAuth,
	// 	cfg.AUTH.Timeout,
	// 	cfg.AUTH.RetriesCount)
	// if err != nil {
	// 	log.Error("error with launch authclient")
	// }

	//создание бд, да плохо
	storage, err := postgresql.New(cfg.DB_URL, log)
	if err != nil {
		panic(err)
	}
	httpApi := http.NewAPI(log, storage)
	// wsHandler := handlers.NewWebSocketHandler(*authClient, neuralClient)
	wsHandler := handlers.NewWebSocketHandler(neuralClient)
	//здесь создание создание http.Api handler
	app := ws.New(log, cfg, wsHandler, httpApi)

	go app.MustRun()

	// TODO: сделать корректную обработку сигналов для остановки grpc serv
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT) // приходят сигналы и запысиваются в канал stop

	sign := <-stop
	log.Info("stopping application", slog.String("signal", sign.String()))

	log.Info("application stopped")

}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = setupPrettySlog()
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	default: // If env config is invalid, set prod settings by default due to security
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}

func setupPrettySlog() *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}

	handler := opts.NewPrettyHandler(os.Stdout)

	return slog.New(handler)
}
