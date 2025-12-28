package config

import (
	"os"
	"strconv"
)

type DBConfig struct {
	Driver   string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

type ServerConfig struct {
	Port                int
	GracefulShutdownSec int
}

type AppConfig struct {
	DB       DBConfig
	Server   ServerConfig
	Auth     AuthConfig
	LiveKit  LiveKitConfig
	AWS      AWSConfig
	Gemini   GeminiConfig
	LLM      LLMConfig
	Speech   SpeechConfig
	LogLevel string
	Env      string
}

type AuthConfig struct {
	JwksURL string
}

type LiveKitConfig struct {
	Host      string
	APIKey    string
	APISecret string
}

type AWSConfig struct {
	AccessKey string
	SecretKey string
	Region    string
	Bucket    string
}

type GeminiConfig struct {
	RealtimeModel string
	ChatModel     string
	APIKey        string
}

type LLMConfig struct {
	Provider string // "ollama", "gemini", or "nvidia"
	Host     string // Provider host or base URL
	Model    string // Model name (e.g., "llama3.2", "qwen2.5")
	APIKey   string // API key for providers that require it (e.g., Nvidia)
}

type SpeechConfig struct {
	Host string // gRPC host:port for Python speech service
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func LoadConfig() (*AppConfig, error) {
	portStr := os.Getenv("DB_PORT")
	portInt, err := strconv.Atoi(portStr)
	if err != nil {
		portInt = 5432
	}
	provider := getEnvOrDefault("LLM_PROVIDER", "ollama")
	defaultLLMHost := "http://localhost:11434"
	defaultLLMModel := "llama3.2"
	if provider == "nvidia" {
		defaultLLMHost = "https://integrate.api.nvidia.com/v1/chat/completions"
		defaultLLMModel = "meta/llama-4-maverick-17b-128e-instruct"
	}
	config := &AppConfig{
		DB: DBConfig{
			Driver:   os.Getenv("DB_DRIVER"),
			Host:     os.Getenv("DB_HOST"),
			Port:     portInt,
			User:     os.Getenv("DB_USERNAME"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_DATABASE"),
		},
		Server: ServerConfig{
			Port:                9000,
			GracefulShutdownSec: 5,
		},
		Auth: AuthConfig{
			JwksURL: os.Getenv("JWKS_URL"),
		},
		LiveKit: LiveKitConfig{
			Host:      os.Getenv("LK_HOST"),
			APIKey:    os.Getenv("LK_API_KEY"),
			APISecret: os.Getenv("LK_API_SECRET"),
		},
		AWS: AWSConfig{
			AccessKey: os.Getenv("AWS_ACCESS_KEY"),
			SecretKey: os.Getenv("AWS_SECRET_KEY"),
			Region:    os.Getenv("AWS_REGION"),
			Bucket:    os.Getenv("AWS_S3_BUCKET"),
		},
		Gemini: GeminiConfig{
			RealtimeModel: os.Getenv("GEMINI_REALTIME_MODEL"),
			ChatModel:     os.Getenv("GEMINI_CHAT_MODEL"),
			APIKey:        os.Getenv("GEMINI_API_KEY"),
		},
		Speech: SpeechConfig{
			Host: getEnvOrDefault("SPEECH_SERVICE_HOST", "localhost:50051"),
		},
		LLM: LLMConfig{
			Provider: provider,
			Host:     getEnvOrDefault("LLM_HOST", defaultLLMHost),
			Model:    getEnvOrDefault("LLM_MODEL", defaultLLMModel),
			APIKey:   os.Getenv("LLM_API_KEY"),
		},
		LogLevel: "info",
		Env:      os.Getenv("APP_ENV"),
	}
	return config, nil
}
