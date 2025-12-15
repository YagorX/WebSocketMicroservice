package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	ENV          string         `yaml:"env" env-default:"local"`
	PORT         string         `yaml:"port"`
	AUTH         AuthGRPCConfig `yaml:"auth"`
	WEBSOCKET    WebSocket      `yaml:"websocket"`
	NEURALCLIENT NeuralClient   `yaml:"neuralclient"`
}

type AuthGRPCConfig struct {
	URLAuth      string        `yaml:"URLAuth"`
	Timeout      time.Duration `yaml:"timeout"`
	RetriesCount int           `yaml:"retriesCount"`
	Insecure     bool          `yaml:"insecure"`
}

type BatcherConfig struct {
	MaxBatchSize int           `env:"MAX_BATCH_SIZE" default:"10"`
	BatchTimeout time.Duration `env:"BATCH_TIMEOUT" default:"2s"`
	WorkerCount  int           `env:"WORKER_COUNT" default:"5"`
}

// стуктура для соединения с фронтом, указать port на котором фронт
type WebSocket struct {
	URLWS   string        `yaml:"urlws"`
	Timeout time.Duration `yaml:"timeout"`
}

// структура для соединения с беком нейронки
type NeuralClient struct {
	URLNeural string        `yaml:"URLNeural"`
	Timeout   time.Duration `yaml:"timeout"`
}

// парсит и возвращает объект конфига
func MustLoadByPath(configPath string) *Config {

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("failed tp read config: " + err.Error())
	}

	return &cfg
}

// парсит и возвращает объект конфига
func MustLoad() *Config {
	path := fetchConfigPath()
	if path == "" {
		panic("config path is empty")
	}

	return MustLoadByPath(path)
}

// получает информацию о пути до файла конфига
// из двух источников либо из переменных окружения
// либо из флага (приоритет)
func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
